#!/usr/bin/env python3
"""Converts the round 3 models that were not published as ONNX, or only in full precision,
into the files attached to the GitHub release `models-2026-10` (internal/dataset/aimodels.go).

Run it once on any machine; the server never runs Python. Anyone can repeat it from the
original weights and compare checksums of the inputs (printed) and the outputs.

    pip install torch transformers onnx onnxruntime safetensors
    python tools/models/convert.py <downloads> <out>

<downloads> holds the original files, one folder per Hugging Face repository named
owner__repo, each file named by its path with / replaced by __, downloaded at the
revisions and checksums pinned in internal/dataset/aimodels.go (and, for the PyTorch
weights only this script reads, at the same revisions: unitary pytorch_model.bin and
config.json, darelphilip model.safetensors and config.json, seanbenhur pytorch_model.bin
and config.json, IndicXlit's en-indic checkpoint and dictionaries).

What it makes:

  xlit-en-indic-encoder.onnx, xlit-en-indic-decoder.onnx, xlit-en-indic-vocabulary.json
      IndicXlit English to Indic v1.0 (AI4Bharat, MIT). The fairseq checkpoint is loaded
      into a plain PyTorch copy of fairseq's transformer (below) and exported. The decoder
      takes everything decoded so far and has no cache: words are short.
  names-indicner-int8.onnx, names-bert-base-ner-int8.onnx
      published ONNX files, quantised to int8 (dynamic quantisation of the weights).
  moderation-unitary-xlmr-int8.onnx, moderation-hinglish-int8.onnx, moderation-tanglish-int8.onnx
      published PyTorch weights, exported to ONNX and quantised to int8. The Tanglish
      model's own ONNX file lacks the classification layer, so its PyTorch weights are used:
      MuRIL, the mean of its token vectors, and one linear output whose sigmoid is the
      probability that the text is offensive (MeanPoolingModel in the author's repository).
  translation-{en-indic,indic-en}-{encoder,decoder,decoder-past}.onnx
      the int8 IndicTrans2 ONNX files of hari31416, with their weights moved from the
      shared external data file into each model, so each is one self-contained file.

Licence of this script: MIT-0.
"""

import hashlib
import json
import math
import os
import shutil
import sys
import tempfile

import onnx
import torch
from onnxruntime.quantization import QuantType, quantize_dynamic


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for block in iter(lambda: f.read(1 << 20), b""):
            h.update(block)
    return h.hexdigest()


def quantise(source, target):
    quantize_dynamic(source, target, weight_type=QuantType.QInt8, per_channel=False)


# --- IndicXlit ------------------------------------------------------------------------

class Attention(torch.nn.Module):
    def __init__(self, dim, heads):
        super().__init__()
        self.heads, self.dim = heads, dim
        self.k_proj = torch.nn.Linear(dim, dim)
        self.v_proj = torch.nn.Linear(dim, dim)
        self.q_proj = torch.nn.Linear(dim, dim)
        self.out_proj = torch.nn.Linear(dim, dim)

    def forward(self, query, key, mask):
        # query [B, T, D], key [B, S, D]; mask [B, 1 or T, S] with True where attention is allowed.
        b, t, d = query.shape
        s = key.shape[1]
        h, hd = self.heads, d // self.heads
        q = (self.q_proj(query) * hd ** -0.5).view(b, t, h, hd).transpose(1, 2)
        k = self.k_proj(key).view(b, s, h, hd).transpose(1, 2)
        v = self.v_proj(key).view(b, s, h, hd).transpose(1, 2)
        scores = q @ k.transpose(-1, -2)
        scores = scores.masked_fill(~mask.unsqueeze(1), float("-inf"))
        out = torch.softmax(scores, dim=-1) @ v
        return self.out_proj(out.transpose(1, 2).reshape(b, t, d))


class EncoderLayer(torch.nn.Module):
    def __init__(self, dim, heads, ffn):
        super().__init__()
        self.self_attn = Attention(dim, heads)
        self.self_attn_layer_norm = torch.nn.LayerNorm(dim)
        self.fc1 = torch.nn.Linear(dim, ffn)
        self.fc2 = torch.nn.Linear(ffn, dim)
        self.final_layer_norm = torch.nn.LayerNorm(dim)

    def forward(self, x, mask):
        y = self.self_attn_layer_norm(x)
        x = x + self.self_attn(y, y, mask)
        y = self.final_layer_norm(x)
        return x + self.fc2(torch.nn.functional.gelu(self.fc1(y)))


class DecoderLayer(torch.nn.Module):
    def __init__(self, dim, heads, ffn):
        super().__init__()
        self.self_attn = Attention(dim, heads)
        self.self_attn_layer_norm = torch.nn.LayerNorm(dim)
        self.encoder_attn = Attention(dim, heads)
        self.encoder_attn_layer_norm = torch.nn.LayerNorm(dim)
        self.fc1 = torch.nn.Linear(dim, ffn)
        self.fc2 = torch.nn.Linear(ffn, dim)
        self.final_layer_norm = torch.nn.LayerNorm(dim)

    def forward(self, x, memory, self_mask, memory_mask):
        y = self.self_attn_layer_norm(x)
        x = x + self.self_attn(y, y, self_mask)
        y = self.encoder_attn_layer_norm(x)
        x = x + self.encoder_attn(y, memory, memory_mask)
        y = self.final_layer_norm(x)
        return x + self.fc2(torch.nn.functional.gelu(self.fc1(y)))


def sinusoidal(positions, dim):
    """fairseq's SinusoidalPositionalEmbedding for the given (already offset) positions."""
    half = dim // 2
    scale = math.log(10000) / (half - 1)
    freqs = torch.exp(torch.arange(half, dtype=torch.float32) * -scale)
    angles = positions.to(torch.float32).unsqueeze(-1) * freqs
    return torch.cat([torch.sin(angles), torch.cos(angles)], dim=-1)


class Encoder(torch.nn.Module):
    def __init__(self, vocab, dim, heads, ffn, layers):
        super().__init__()
        self.dim = dim
        self.embed_tokens = torch.nn.Embedding(vocab, dim)
        self.layernorm_embedding = torch.nn.LayerNorm(dim)
        self.layers = torch.nn.ModuleList([EncoderLayer(dim, heads, ffn) for _ in range(layers)])
        self.layer_norm = torch.nn.LayerNorm(dim)

    def forward(self, input_ids, attention_mask):
        # Positions as fairseq numbers them: from padding index + 1 = 2, over real tokens.
        positions = torch.cumsum(attention_mask, dim=1) * attention_mask + 1
        x = self.embed_tokens(input_ids) * math.sqrt(self.dim) + sinusoidal(positions, self.dim)
        x = self.layernorm_embedding(x)
        mask = attention_mask.bool().unsqueeze(1)
        for layer in self.layers:
            x = layer(x, mask)
        return self.layer_norm(x)


class Decoder(torch.nn.Module):
    def __init__(self, vocab, dim, heads, ffn, layers):
        super().__init__()
        self.dim = dim
        self.embed_tokens = torch.nn.Embedding(vocab, dim)
        self.layernorm_embedding = torch.nn.LayerNorm(dim)
        self.layers = torch.nn.ModuleList([DecoderLayer(dim, heads, ffn) for _ in range(layers)])
        self.layer_norm = torch.nn.LayerNorm(dim)
        self.output_projection = torch.nn.Linear(dim, vocab, bias=False)

    def forward(self, input_ids, encoder_hidden_states, encoder_attention_mask):
        t = input_ids.shape[1]
        positions = torch.arange(t, dtype=torch.int64).unsqueeze(0).expand_as(input_ids) + 2
        x = self.embed_tokens(input_ids) * math.sqrt(self.dim) + sinusoidal(positions, self.dim)
        x = self.layernorm_embedding(x)
        causal = torch.tril(torch.ones(t, t, dtype=torch.bool)).unsqueeze(0)
        memory_mask = encoder_attention_mask.bool().unsqueeze(1)
        for layer in self.layers:
            x = layer(x, encoder_hidden_states, causal, memory_mask)
        return self.output_projection(self.layer_norm(x))


def fairseq_checkpoint(path):
    """Loads a fairseq checkpoint without fairseq: any class it names becomes a stand-in."""
    import pickle
    import types

    class StandIn:
        def __init__(self, *args, **kwargs):
            pass

        def __setstate__(self, state):
            self.__dict__.update(state if isinstance(state, dict) else {"state": state})

    class Unpickler(pickle.Unpickler):
        def find_class(self, module, name):
            try:
                return super().find_class(module, name)
            except Exception:
                return StandIn

    module = types.SimpleNamespace(Unpickler=Unpickler, load=pickle.load, __name__="pickle")
    return torch.load(path, map_location="cpu", weights_only=False, pickle_module=module)


def fairseq_dictionary(path, languages):
    """fairseq's Dictionary: four special symbols, the file's symbols, then language tags."""
    symbols = ["<s>", "<pad>", "</s>", "<unk>"]
    with open(path, encoding="utf-8") as f:
        for line in f:
            symbol = line.rstrip("\n").rsplit(" ", 1)[0]
            symbols.append(symbol)
    symbols += ["__%s__" % lang for lang in languages]
    return symbols


XLIT_LANGUAGES = "en as bn brx gom gu hi kn ks mai ml mni mr ne or pa sa sd si ta te ur".split()


def convert_xlit(downloads, out):
    folder = os.path.join(downloads, "ai4bharat__IndicXlit")
    checkpoint = fairseq_checkpoint(os.path.join(folder, "indicxlit-en-indic-v1.0__transformer__indicxlit.pt"))
    weights = checkpoint["model"]
    source = fairseq_dictionary(os.path.join(folder, "indicxlit-en-indic-v1.0__corpus-bin__dict.en.txt"), XLIT_LANGUAGES)
    target = fairseq_dictionary(os.path.join(folder, "indicxlit-en-indic-v1.0__corpus-bin__dict.ta.txt"), XLIT_LANGUAGES)
    assert len(source) == weights["encoder.embed_tokens.weight"].shape[0], "source dictionary size"
    assert len(target) == weights["decoder.embed_tokens.weight"].shape[0], "target dictionary size"

    encoder = Encoder(len(source), 256, 4, 1024, 6)
    decoder = Decoder(len(target), 256, 4, 1024, 6)
    for model, prefix in ((encoder, "encoder."), (decoder, "decoder.")):
        state = {k[len(prefix):]: v for k, v in weights.items()
                 if k.startswith(prefix) and "embed_positions" not in k and not k.endswith(".version")}
        model.load_state_dict(state, strict=True)
        model.eval()

    ids = torch.tensor([[source.index("__ta__")] + [source.index(c) for c in "vanakkam"] + [2]])
    mask = torch.ones_like(ids)
    with torch.no_grad():
        memory = encoder(ids, mask)
        seq = torch.tensor([[2]])
        for _ in range(20):
            logits = decoder(seq, memory, mask)
            nxt = int(logits[0, -1].argmax())
            if nxt == 2:
                break
            seq = torch.cat([seq, torch.tensor([[nxt]])], dim=1)
    word = "".join(target[i] for i in seq[0, 1:].tolist())
    print("xlit check: vanakkam ->", word)
    assert word == "வணக்கம்", word

    torch.onnx.export(encoder, (ids, mask), os.path.join(out, "xlit-en-indic-encoder.onnx"),
                      input_names=["input_ids", "attention_mask"], output_names=["last_hidden_state"],
                      dynamic_axes={"input_ids": {0: "batch", 1: "source"}, "attention_mask": {0: "batch", 1: "source"},
                                    "last_hidden_state": {0: "batch", 1: "source"}},
                      opset_version=17, dynamo=False)
    torch.onnx.export(decoder, (seq, memory, mask), os.path.join(out, "xlit-en-indic-decoder.onnx"),
                      input_names=["input_ids", "encoder_hidden_states", "encoder_attention_mask"], output_names=["logits"],
                      dynamic_axes={"input_ids": {0: "batch", 1: "target"}, "encoder_hidden_states": {0: "batch", 1: "source"},
                                    "encoder_attention_mask": {0: "batch", 1: "source"}, "logits": {0: "batch", 1: "target"}},
                      opset_version=17, dynamo=False)
    with open(os.path.join(out, "xlit-en-indic-vocabulary.json"), "w", encoding="utf-8") as f:
        json.dump({"source": source, "target": target, "languages": XLIT_LANGUAGES[1:],
                   "start": 2, "end": 2, "pad": 1, "unk": 3}, f, ensure_ascii=False)


# --- Classifiers ----------------------------------------------------------------------

def export_classifier(model, out_path, uses_types):
    model.eval()
    ids = torch.tensor([[0, 100, 200, 2]])
    mask = torch.ones_like(ids)
    inputs = (ids, mask) if not uses_types else (ids, mask, torch.zeros_like(ids))
    names = ["input_ids", "attention_mask"] + (["token_type_ids"] if uses_types else [])
    axes = {n: {0: "batch", 1: "sequence"} for n in names}
    axes["logits"] = {0: "batch"}

    class Logits(torch.nn.Module):
        def __init__(self, inner):
            super().__init__()
            self.inner = inner

        def forward(self, *args):
            return self.inner(*args).logits

    torch.onnx.export(Logits(model), inputs, out_path, input_names=names, output_names=["logits"],
                      dynamic_axes=axes, opset_version=17, dynamo=False)


def convert_classifiers(downloads, out, work):
    import types
    from transformers import AutoConfig, BertModel, XLMRobertaForSequenceClassification
    from safetensors.torch import load_file

    # Unitary's multilingual toxicity model: PyTorch weights and a config.
    folder = os.path.join(downloads, "unitary__multilingual-toxic-xlm-roberta")
    config = AutoConfig.from_pretrained(os.path.join(folder, "config.json"))
    model = XLMRobertaForSequenceClassification(config)
    state = torch.load(os.path.join(folder, "pytorch_model.bin"), map_location="cpu", weights_only=True)
    missing, unexpected = model.load_state_dict(state, strict=False)
    assert not [m for m in missing if "position_ids" not in m], missing
    full = os.path.join(work, "unitary.onnx")
    export_classifier(model, full, uses_types=False)
    quantise(full, os.path.join(out, "moderation-unitary-xlmr-int8.onnx"))

    # The Hinglish toxicity classifier: safetensors and a config.
    folder = os.path.join(downloads, "darelphilip__hinglish-toxicity-classifier")
    config = AutoConfig.from_pretrained(os.path.join(folder, "config.json"))
    model = XLMRobertaForSequenceClassification(config)
    missing, unexpected = model.load_state_dict(load_file(os.path.join(folder, "model.safetensors")), strict=False)
    assert not [m for m in missing if "position_ids" not in m], missing
    full = os.path.join(work, "hinglish.onnx")
    export_classifier(model, full, uses_types=False)
    quantise(full, os.path.join(out, "moderation-hinglish-int8.onnx"))

    # The Tanglish model: MuRIL with mean pooling and one output.
    folder = os.path.join(downloads, "seanbenhur__tanglish-offensive-language-identification")
    config = AutoConfig.from_pretrained(os.path.join(folder, "config.json"))

    class MeanPooling(torch.nn.Module):
        def __init__(self):
            super().__init__()
            self.model = BertModel(config, add_pooling_layer=True)
            self.linear = torch.nn.Linear(config.hidden_size, 1)

        def forward(self, input_ids, attention_mask):
            hidden = self.model(input_ids=input_ids, attention_mask=attention_mask)[0]
            mask = attention_mask.unsqueeze(-1).to(hidden.dtype)
            mean = (hidden * mask).sum(1) / mask.sum(1).clamp(min=1e-9)
            return types.SimpleNamespace(logits=self.linear(mean))

    model = MeanPooling()
    state = torch.load(os.path.join(folder, "pytorch_model.bin"), map_location="cpu", weights_only=True)
    state = {k: v for k, v in state.items() if not k.endswith("position_ids")}
    model.load_state_dict(state, strict=True)
    full = os.path.join(work, "tanglish.onnx")
    export_classifier(model, full, uses_types=False)
    quantise(full, os.path.join(out, "moderation-tanglish-int8.onnx"))

    # Published ONNX files, only quantised.
    for source, target in (
        ("abbasrazalaghari__indicner-onnx/model.onnx", "names-indicner-int8.onnx"),
        ("dslim__bert-base-NER/onnx__model.onnx", "names-bert-base-ner-int8.onnx"),
    ):
        quantise(os.path.join(downloads, source), os.path.join(out, target))


# --- IndicTrans2 ------------------------------------------------------------------------

def convert_translation(downloads, out):
    for direction, repo in (("en-indic", "hari31416__indictrans2-en-indic-dist-200M-ONNX-int8"),
                            ("indic-en", "hari31416__indictrans2-indic-en-dist-200M-ONNX-int8")):
        folder = os.path.join(downloads, repo)
        for source, part in (("encoder_model.onnx", "encoder"), ("decoder_model.onnx", "decoder"),
                             ("decoder_with_past_model.onnx", "decoder-past")):
            model = onnx.load(os.path.join(folder, source), load_external_data=True)
            onnx.save(model, os.path.join(out, "translation-%s-%s.onnx" % (direction, part)))


def main():
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    downloads, out = sys.argv[1], sys.argv[2]
    os.makedirs(out, exist_ok=True)
    only = os.environ.get("ONLY", "xlit,classifiers,translation").split(",")
    work = tempfile.mkdtemp()
    try:
        if "xlit" in only:
            convert_xlit(downloads, out)
        if "translation" in only:
            convert_translation(downloads, out)
        if "classifiers" in only:
            convert_classifiers(downloads, out, work)
    finally:
        shutil.rmtree(work, ignore_errors=True)
    for name in sorted(os.listdir(out)):
        path = os.path.join(out, name)
        print("%s  %s  %d" % (sha256(path), name, os.path.getsize(path)))


if __name__ == "__main__":
    main()
