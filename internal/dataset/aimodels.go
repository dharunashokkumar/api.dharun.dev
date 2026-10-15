package dataset

import (
	"fmt"
	"time"
)

// The AI models of round 3: embeddings, reranking, speech to text, name detection,
// moderation, translation and transliteration. Like the voices and the background
// removal model, each is fetched by a server, pinned by revision and checksum, and never
// kept in the repository or a release bundle.
//
// Where a model was published only as PyTorch weights, or in full precision only, it was
// converted to ONNX (and quantised to int8) once with tools/models/convert.py, and the
// result is attached to this repository's GitHub release ModelsRelease. The script is
// committed, so anyone can repeat the conversion from the original weights and compare.

// ModelsRelease is the GitHub release that holds the converted model files.
const ModelsRelease = "models-2026-10"

const releaseBase = "https://github.com/dharunashokkumar/api.dharun.dev/releases/download/" + ModelsRelease + "/"

func hf(repo, revision, file string) string {
	return "https://huggingface.co/" + repo + "/resolve/" + revision + "/" + file
}

// modelFile is one file of a model pack, attached to the dataset under Name.
type modelFile struct {
	Name   string // the attachment's kind, such as "model" or "tokenizer"
	URL    string
	SHA256 string
	Ext    string // the extension the attachment keeps: ".onnx", ".json", ".bin"
}

// modelPack is a dataset made only of model files.
type modelPack struct {
	name    string
	version string // changes whenever a file changes
	files   []modelFile
	source  Source
	models  int
}

func registerModelPack(pack modelPack) {
	register(&Spec{
		Name:   pack.name,
		Schema: 1,
		Build: func(b *Builder) error {
			for _, f := range pack.files {
				if err := b.Attach(b.Path(f.Name+f.Ext), f.Name); err != nil {
					return fmt.Errorf("%s: %w", f.Name, err)
				}
			}
			b.Count("models", pack.models)
			return nil
		},
		Import: func(im *Importer) (Source, error) {
			if im.CurrentVersion() == pack.version {
				return Source{}, ErrNotChanged
			}
			for _, f := range pack.files {
				path, err := im.Download(f.URL, f.Name+f.Ext)
				if err != nil {
					return Source{}, err
				}
				if err := checkDigest(path, f.SHA256); err != nil {
					return Source{}, fmt.Errorf("%s: %w", f.Name, err)
				}
			}
			source := pack.source
			source.Version = pack.version
			return source, nil
		},
		Every: 30 * 24 * time.Hour,
		Main:  "models", MaxShrink: 0,
		NoSeed: true,
	})
}

// Revisions of the Hugging Face repositories the files come from.
const (
	e5Revision       = "614241f622f53c4eeff9890bdc4f31cfecc418b3"
	rerankerRevision = "6f5ff65298512715a1e669753bc754d2bc8f367b"
	whisperRevision  = "5359861c739e955e79d9a303bcbc70fb988958b1"
	indicNERRevision = "e224f9c674a17890fd9f9a4b1242245979dbf522"
	bertNERRevision  = "d1a3e8f13f8c3566299d95fcfc9a8d2382a9affc"
	hinglishRevision = "0c069e60ff1eb3451489850e84076b4154865766"
	tanglishRevision = "06bbb3a64eb8f563435c4bceb0bad480e70ac11e"
	enIndicRevision  = "1eaa43a3fb76f382f1b22c76e56f6ab4926ad68b"
	indicEnRevision  = "a7ecdc0ba73148c6e1d051ca372332471d8aff4f"
)

func init() {
	registerModelPack(modelPack{
		name:    "embeddings",
		version: "multilingual-e5-small@" + e5Revision[:7],
		models:  1,
		files: []modelFile{
			{"model", hf("intfloat/multilingual-e5-small", e5Revision, "onnx/model_qint8_avx512_vnni.onnx"), "dd476dd0c2514e9b9be83aeb3853fac0763e0bdf4a71645407587d77c48a2d88", ".onnx"},
			{"tokenizer", hf("intfloat/multilingual-e5-small", e5Revision, "onnx/tokenizer.json"), "0b44a9d7b51c3c62626640cda0e2c2f70fdacdc25bbbd68038369d14ebdf4c39", ".json"},
		},
		source: Source{
			Name:        "multilingual-e5-small (int8)",
			Publisher:   "Liang Wang et al., Microsoft",
			URL:         "https://huggingface.co/intfloat/multilingual-e5-small",
			Licence:     "MIT",
			LicenceURL:  "https://huggingface.co/intfloat/multilingual-e5-small",
			Attribution: "Embeddings by multilingual-e5-small (Wang et al., Multilingual E5 Text Embeddings, 2024), MIT.",
		},
	})
	registerModelPack(modelPack{
		name:    "reranker",
		version: "bge-reranker-v2-m3@" + rerankerRevision[:7],
		models:  1,
		files: []modelFile{
			{"model", hf("onnx-community/bge-reranker-v2-m3-ONNX", rerankerRevision, "onnx/model_int8.onnx"), "912fc1215c2dbff6499700534bd8d31253af01573861abbfc43afd1fab6cce5d", ".onnx"},
			{"tokenizer", hf("onnx-community/bge-reranker-v2-m3-ONNX", rerankerRevision, "tokenizer.json"), "8bf8afbfd11306bd872018c53bfdf2e160a56f8edbcf49933324404791c148d3", ".json"},
		},
		source: Source{
			Name:        "bge-reranker-v2-m3 (int8)",
			Publisher:   "BAAI; ONNX export by onnx-community",
			URL:         "https://huggingface.co/BAAI/bge-reranker-v2-m3",
			Licence:     "Apache-2.0",
			LicenceURL:  "https://huggingface.co/BAAI/bge-reranker-v2-m3",
			Attribution: "Reranking by bge-reranker-v2-m3 (BAAI), Apache-2.0.",
		},
	})
	registerModelPack(modelPack{
		name:    "whisper",
		version: "whisper-small-q5_1@" + whisperRevision[:7],
		models:  1,
		files: []modelFile{
			{"model", hf("ggerganov/whisper.cpp", whisperRevision, "ggml-small-q5_1.bin"), "ae85e4a935d7a567bd102fe55afc16bb595bdb618e11b2fc7591bc08120411bb", ".bin"},
		},
		source: Source{
			Name:        "Whisper small (5-bit, whisper.cpp)",
			Publisher:   "OpenAI; converted by the whisper.cpp project",
			URL:         "https://github.com/openai/whisper",
			Licence:     "MIT",
			LicenceURL:  "https://github.com/openai/whisper/blob/main/LICENSE",
			Attribution: "Speech recognition by OpenAI Whisper small (MIT), run with whisper.cpp (MIT).",
		},
	})
	registerModelPack(modelPack{
		name:    "names",
		version: "indicner+bert-base-ner@" + ModelsRelease,
		models:  2,
		files: []modelFile{
			{"indic", releaseBase + "names-indicner-int8.onnx", "b0f42dc2b1f52dbe96d5c55eac301bc76099ef4ab36c24d696d2cd16f4667efa", ".onnx"},
			{"indic-tokenizer", hf("abbasrazalaghari/indicner-onnx", indicNERRevision, "tokenizer.json"), "ce6b3bc23946dfbbdf84cf044e0a29b0378b3783e7d18b1c2339de5da4840056", ".json"},
			{"english", releaseBase + "names-bert-base-ner-int8.onnx", "657dbf2ecc969a828fab77e2b83b2470f0b7e39261b163e2994fdb6ab05745d5", ".onnx"},
			{"english-tokenizer", hf("dslim/bert-base-NER", bertNERRevision, "onnx/tokenizer.json"), "343989712a36cd8b253efeaf8baf6a08b9d2583f78e395e83832e8ee9f8d8ee1", ".json"},
		},
		source: Source{
			Name:        "IndicNER and bert-base-NER (int8)",
			Publisher:   "AI4Bharat (IndicNER); David S. Lim (bert-base-NER)",
			URL:         "https://huggingface.co/ai4bharat/IndicNER",
			Licence:     "MIT",
			LicenceURL:  "https://huggingface.co/ai4bharat/IndicNER",
			Attribution: "Names in Indian scripts by IndicNER (AI4Bharat, Mhaske et al., Naamapadam, 2023), MIT; in English by bert-base-NER (David S. Lim), MIT.",
		},
	})
	registerModelPack(modelPack{
		name:    "moderation",
		version: "unitary+hinglish+tanglish@" + ModelsRelease,
		models:  3,
		files: []modelFile{
			{"general", releaseBase + "moderation-unitary-xlmr-int8.onnx", "0d4f470db0329b8b904e20a66e7839bc93b65e45e91f7f5f82dc94c890b33148", ".onnx"},
			{"general-tokenizer", hf("intfloat/multilingual-e5-small", e5Revision, "onnx/tokenizer.json"), "0b44a9d7b51c3c62626640cda0e2c2f70fdacdc25bbbd68038369d14ebdf4c39", ".json"},
			{"hinglish", releaseBase + "moderation-hinglish-int8.onnx", "1472dd99a38757fc153e3abd65a5a0cf9565b08a2a7e46573fecfefb1f80633b", ".onnx"},
			{"hinglish-tokenizer", hf("darelphilip/hinglish-toxicity-classifier", hinglishRevision, "tokenizer.json"), "3a56def25aa40facc030ea8b0b87f3688e4b3c39eb8b45d5702b3a1300fe2a20", ".json"},
			{"tanglish", releaseBase + "moderation-tanglish-int8.onnx", "f4ed19c7620b0647d451d41b3b66073b85fdc327acb3a517cb62a0ba4a0217f6", ".onnx"},
			{"tanglish-tokenizer", hf("seanbenhur/tanglish-offensive-language-identification", tanglishRevision, "tokenizer.json"), "3e96bdce8be537286dc60fc5e79b938b87afc999ea91f2ecb46622907f171de3", ".json"},
		},
		source: Source{
			Name:        "Toxicity classifiers (int8): Unitary multilingual, Hinglish, Tanglish",
			Publisher:   "Unitary (Detoxify); Darel Philip; Sean Benhur",
			URL:         "https://huggingface.co/unitary/multilingual-toxic-xlm-roberta",
			Licence:     "Apache-2.0 (Unitary, Tanglish); MIT (Hinglish, base model L3Cube HingRoBERTa CC BY 4.0)",
			LicenceURL:  "https://github.com/unitaryai/detoxify/blob/master/LICENSE",
			Attribution: "Moderation by Detoxify multilingual (Unitary, Apache-2.0), the Hinglish toxicity classifier by Darel Philip (MIT; built on L3Cube HingRoBERTa, CC BY 4.0) and the Tanglish offensive language model by Sean Benhur (Apache-2.0).",
		},
	})
	registerModelPack(modelPack{
		name:    "translation",
		version: "indictrans2-dist-200M@" + ModelsRelease,
		models:  2,
		files: []modelFile{
			{"en-indic-encoder", releaseBase + "translation-en-indic-encoder.onnx", "0f2dd6c8929655855afd12d143209d493b4da46b961101d58edc31d017d2978c", ".onnx"},
			{"en-indic-decoder", releaseBase + "translation-en-indic-decoder.onnx", "c95e0fec2df4387e246a9b70f9c6062a0ed2f1c3223006c429c07333bc2731b3", ".onnx"},
			{"en-indic-decoder-past", releaseBase + "translation-en-indic-decoder-past.onnx", "132998d8ad76a439c386546a56e825a066c443e531e4b285cce6b8ab435e63da", ".onnx"},
			{"en-indic-source", hf("hari31416/indictrans2-en-indic-dist-200M-ONNX-int8", enIndicRevision, "tokenizer_src.json"), "ac1930c411028ba3a7929f35bc7f62d2e5e837ca24150b661de4f4606469eeca", ".json"},
			{"en-indic-target", hf("hari31416/indictrans2-en-indic-dist-200M-ONNX-int8", enIndicRevision, "tokenizer_tgt.json"), "51b05cd37636765e158146534d54452be558df32c6ab06ba4caeecdd0266a580", ".json"},
			{"indic-en-encoder", releaseBase + "translation-indic-en-encoder.onnx", "a6e46e6bf6af06e3ed49244a86a4e226b04f28571df0a83cf8dd16856f07546d", ".onnx"},
			{"indic-en-decoder", releaseBase + "translation-indic-en-decoder.onnx", "5756a9c7d2c4171dc935cb81f1f6da8cf009d23dde9ccb7939d2a941b8bda51c", ".onnx"},
			{"indic-en-decoder-past", releaseBase + "translation-indic-en-decoder-past.onnx", "aef9caa92e2e555425f01edc308b3a0d4efbef3be5d72afcc9be3b2d6fa4d847", ".onnx"},
			{"indic-en-source", hf("hari31416/indictrans2-indic-en-dist-200M-ONNX-int8", indicEnRevision, "tokenizer_src.json"), "d15ba1265a61b3b66122aab74e888442093a2c711f51b0f9896108661e1d346c", ".json"},
			{"indic-en-target", hf("hari31416/indictrans2-indic-en-dist-200M-ONNX-int8", indicEnRevision, "tokenizer_tgt.json"), "053e948b03403ad9adc43a08e7e5c5d83854fffeddf1aae85e3ca82258698575", ".json"},
		},
		source: Source{
			Name:        "IndicTrans2 distilled 200M (int8)",
			Publisher:   "AI4Bharat; ONNX export by Harikesh Kushwaha",
			URL:         "https://github.com/AI4Bharat/IndicTrans2",
			Licence:     "MIT",
			LicenceURL:  "https://github.com/AI4Bharat/IndicTrans2/blob/main/LICENSE",
			Attribution: "Translation by IndicTrans2 (AI4Bharat, Gala et al., 2023), MIT.",
		},
	})
	registerModelPack(modelPack{
		name:    "xlit",
		version: "indicxlit-en-indic-v1.0@" + ModelsRelease,
		models:  1,
		files: []modelFile{
			{"encoder", releaseBase + "xlit-en-indic-encoder.onnx", "4f1c7b52c78d03dade4dd550c2ce3301e049c4cfcab55a6a4e42a40308bfd65e", ".onnx"},
			{"decoder", releaseBase + "xlit-en-indic-decoder.onnx", "262bf24a1b15727ec61073a604d7f63491e9feaf19ab074d95b734e5ca4f5372", ".onnx"},
			{"vocabulary", releaseBase + "xlit-en-indic-vocabulary.json", "5c26262a78719e488cf69d5199880ad877323f2dbdb7387c7a3721a4e373ad35", ".json"},
		},
		source: Source{
			Name:        "IndicXlit, English to Indic v1.0",
			Publisher:   "AI4Bharat",
			URL:         "https://github.com/AI4Bharat/IndicXlit",
			Licence:     "MIT",
			LicenceURL:  "https://github.com/AI4Bharat/IndicXlit/blob/master/LICENSE",
			Attribution: "Transliteration by IndicXlit (AI4Bharat, Madhani et al., Aksharantar, 2022), MIT.",
		},
	})
}
