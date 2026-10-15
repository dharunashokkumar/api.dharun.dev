//go:build cgo

package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/dharunashokkumar/api.dharun.dev/internal/onnxwire"
	ort "github.com/yalue/onnxruntime_go"
)

// serve keeps models loaded and answers requests from the API on standard input and
// output (internal/onnxwire) until its input closes. The API starts one per kind of
// model and stops it when it has been idle for a while, which gives the memory back.
func serve(threads int) error {
	if err := initRuntime(); err != nil {
		return err
	}
	defer ort.DestroyEnvironment()
	s := &server{threads: max(1, threads), sessions: map[string]*session{}}
	defer s.close()
	return onnxwire.NewConn(os.Stdin, os.Stdout).Serve(s)
}

type session struct {
	run           *ort.DynamicAdvancedSession
	inputs        []ort.InputOutputInfo
	outputs       []ort.InputOutputInfo
	inputIndex    map[string]int
	outputIndex   map[string]int
	outputNames   []string
	inputNameList []string
}

type server struct {
	threads  int
	sessions map[string]*session
}

func (s *server) close() {
	for _, one := range s.sessions {
		one.run.Destroy()
	}
}

func (s *server) session(path string) (*session, error) {
	if one, ok := s.sessions[path]; ok {
		return one, nil
	}
	options, err := ort.NewSessionOptions()
	if err != nil {
		return nil, err
	}
	defer options.Destroy()
	options.SetIntraOpNumThreads(s.threads)
	options.SetInterOpNumThreads(1)
	// Reading the names loads the model once more; without graph optimisation that is
	// quick, and the session that runs it is optimised.
	quick, err := ort.NewSessionOptions()
	if err != nil {
		return nil, err
	}
	quick.SetGraphOptimizationLevel(ort.GraphOptimizationLevelDisableAll)
	inputs, outputs, err := ort.GetInputOutputInfoWithOptions(path, quick)
	quick.Destroy()
	if err != nil {
		return nil, err
	}
	one := &session{inputs: inputs, outputs: outputs, inputIndex: map[string]int{}, outputIndex: map[string]int{}}
	for i, in := range inputs {
		one.inputIndex[in.Name] = i
		one.inputNameList = append(one.inputNameList, in.Name)
	}
	for i, out := range outputs {
		one.outputIndex[out.Name] = i
		one.outputNames = append(one.outputNames, out.Name)
	}
	one.run, err = ort.NewDynamicAdvancedSession(path, one.inputNameList, one.outputNames, options)
	if err != nil {
		return nil, err
	}
	s.sessions[path] = one
	return one, nil
}

// call runs a session with values by name and returns every output by name. The caller
// destroys the outputs.
func (one *session) call(values map[string]ort.Value) (map[string]ort.Value, error) {
	inputs := make([]ort.Value, len(one.inputs))
	for i, in := range one.inputs {
		value, ok := values[in.Name]
		if !ok {
			return nil, fmt.Errorf("input %q is missing", in.Name)
		}
		inputs[i] = value
	}
	outputs := make([]ort.Value, len(one.outputs))
	if err := one.run.Run(inputs, outputs); err != nil {
		return nil, err
	}
	named := make(map[string]ort.Value, len(outputs))
	for i, out := range one.outputs {
		named[out.Name] = outputs[i]
	}
	return named, nil
}

func destroyAll(values map[string]ort.Value) {
	for _, v := range values {
		if v != nil {
			v.Destroy()
		}
	}
}

func newValue(t onnxwire.Tensor) (ort.Value, error) {
	if t.Size() != int64(max(len(t.Floats), len(t.Ints))) {
		return nil, fmt.Errorf("tensor %q holds the wrong number of values", t.Name)
	}
	shape := ort.NewShape(t.Shape...)
	switch t.Type {
	case onnxwire.Float32:
		return ort.NewTensor(shape, t.Floats)
	case onnxwire.Int64:
		return ort.NewTensor(shape, t.Ints)
	}
	return nil, fmt.Errorf("tensor %q has an unknown type", t.Name)
}

func toWire(name string, v ort.Value) (onnxwire.Tensor, error) {
	t := onnxwire.Tensor{Name: name, Shape: slices.Clone([]int64(v.GetShape()))}
	switch value := v.(type) {
	case *ort.Tensor[float32]:
		t.Type, t.Floats = onnxwire.Float32, slices.Clone(value.GetData())
	case *ort.Tensor[int64]:
		t.Type, t.Ints = onnxwire.Int64, slices.Clone(value.GetData())
	default:
		return t, fmt.Errorf("output %q is neither float32 nor int64", name)
	}
	return t, nil
}

func (s *server) Run(req onnxwire.RunRequest) ([]onnxwire.Tensor, error) {
	one, err := s.session(req.Model)
	if err != nil {
		return nil, err
	}
	values := map[string]ort.Value{}
	defer destroyAll(values)
	for _, t := range req.Inputs {
		value, err := newValue(t)
		if err != nil {
			return nil, err
		}
		values[t.Name] = value
	}
	outputs, err := one.call(values)
	if err != nil {
		return nil, err
	}
	defer destroyAll(outputs)
	names := req.Outputs
	if len(names) == 0 {
		names = one.outputNames
	}
	var answer []onnxwire.Tensor
	for _, name := range names {
		value, ok := outputs[name]
		if !ok {
			return nil, fmt.Errorf("the model has no output %q", name)
		}
		t, err := toWire(name, value)
		if err != nil {
			return nil, err
		}
		answer = append(answer, t)
	}
	return answer, nil
}

func (s *server) Generate(req onnxwire.GenerateRequest) ([]onnxwire.Sequence, error) {
	if len(req.Input) == 0 || len(req.Input) > 1024 {
		return nil, errors.New("the input must hold 1 to 1024 tokens")
	}
	if req.MaxLength < 1 || req.MaxLength > 1024 {
		return nil, errors.New("the maximum length must be 1 to 1024")
	}
	encoder, err := s.session(req.Encoder)
	if err != nil {
		return nil, err
	}
	decoder, err := s.session(req.Decoder)
	if err != nil {
		return nil, err
	}
	length := int64(len(req.Input))
	mask := make([]int64, length)
	for i := range mask {
		mask[i] = 1
	}
	ids, err := ort.NewTensor(ort.NewShape(1, length), slices.Clone(req.Input))
	if err != nil {
		return nil, err
	}
	defer ids.Destroy()
	maskValue, err := ort.NewTensor(ort.NewShape(1, length), mask)
	if err != nil {
		return nil, err
	}
	defer maskValue.Destroy()
	encoded, err := encoder.call(map[string]ort.Value{"input_ids": ids, "attention_mask": maskValue})
	if err != nil {
		return nil, err
	}
	defer destroyAll(encoded)
	hidden, ok := encoded["last_hidden_state"].(*ort.Tensor[float32])
	if !ok {
		return nil, errors.New("the encoder has no float32 last_hidden_state")
	}
	banned := map[int64]bool{}
	for _, id := range req.Banned {
		banned[id] = true
	}
	choose := func(logits []float32) []float32 {
		// Log probabilities, with banned and out-of-range tokens never chosen.
		limit := int64(len(logits))
		if req.VocabLimit > 0 && req.VocabLimit < limit {
			limit = req.VocabLimit
		}
		out := make([]float32, len(logits))
		highest := float32(math.Inf(-1))
		for i := int64(0); i < int64(len(logits)); i++ {
			if i >= limit || banned[i] {
				out[i] = float32(math.Inf(-1))
				continue
			}
			out[i] = logits[i]
			highest = max(highest, logits[i])
		}
		var sum float64
		for _, v := range out {
			if !math.IsInf(float64(v), -1) {
				sum += math.Exp(float64(v - highest))
			}
		}
		logSum := float32(math.Log(sum)) + highest
		for i, v := range out {
			out[i] = v - logSum
		}
		return out
	}
	if req.DecoderPast != "" {
		past, err := s.session(req.DecoderPast)
		if err != nil {
			return nil, err
		}
		return greedy(req, decoder, past, hidden, maskValue, choose)
	}
	return beam(req, decoder, hidden, length, choose)
}

// greedy decodes one token at a time, keeping the attention cache between steps.
func greedy(req onnxwire.GenerateRequest, first, past *session, hidden *ort.Tensor[float32], mask *ort.Tensor[int64], choose func([]float32) []float32) ([]onnxwire.Sequence, error) {
	out := []int64{}
	var total float64
	var cache map[string]ort.Value // present.* outputs of the last step
	defer func() { destroyAll(cache) }()
	token := req.Start
	for step := 0; step < req.MaxLength; step++ {
		input, err := ort.NewTensor(ort.NewShape(1, 1), []int64{token})
		if err != nil {
			return nil, err
		}
		values := map[string]ort.Value{"input_ids": input, "encoder_attention_mask": mask}
		run := first
		if step == 0 {
			values["encoder_hidden_states"] = hidden
		} else {
			run = past
			for name, value := range cache {
				if after, ok := strings.CutPrefix(name, "present."); ok {
					values["past_key_values."+after] = value
				}
			}
		}
		results, err := run.call(values)
		input.Destroy()
		if err != nil {
			return nil, err
		}
		logits, ok := results["logits"].(*ort.Tensor[float32])
		if !ok {
			destroyAll(results)
			return nil, errors.New("the decoder has no float32 logits")
		}
		data := logits.GetData()
		vocab := int(logits.GetShape()[len(logits.GetShape())-1])
		scores := choose(data[len(data)-vocab:])
		best := 0
		for i, v := range scores {
			if v > scores[best] {
				best = i
			}
		}
		// Keep the cache: a step with the cache may not give back the encoder's part,
		// which stays as the first step made it.
		next := map[string]ort.Value{}
		for name, value := range results {
			if strings.HasPrefix(name, "present.") {
				next[name] = value
			} else {
				value.Destroy()
			}
		}
		for name, value := range cache {
			if _, replaced := next[name]; replaced {
				value.Destroy()
			} else {
				next[name] = value
			}
		}
		cache = next
		total += float64(scores[best])
		if int64(best) == req.End {
			break
		}
		out = append(out, int64(best))
		token = int64(best)
	}
	return []onnxwire.Sequence{{Score: float32(total / float64(len(out)+1)), IDs: out}}, nil
}

type hypothesis struct {
	ids   []int64
	score float64 // sum of log probabilities
}

// beam runs a beam search, feeding the decoder everything decoded so far at each step.
// Meant for short outputs such as a transliterated word.
func beam(req onnxwire.GenerateRequest, decoder *session, hidden *ort.Tensor[float32], length int64, choose func([]float32) []float32) ([]onnxwire.Sequence, error) {
	width := min(max(req.Beams, 1), 16)
	best := min(max(req.Best, 1), width)
	shape := hidden.GetShape()
	if len(shape) != 3 {
		return nil, errors.New("the encoder's output is not three-dimensional")
	}
	size := shape[2]
	states := hidden.GetData()
	live := []hypothesis{{ids: []int64{req.Start}}}
	var done []hypothesis
	for step := 0; step < req.MaxLength && len(live) > 0; step++ {
		n := int64(len(live))
		t := int64(len(live[0].ids))
		ids := make([]int64, 0, n*t)
		for _, h := range live {
			ids = append(ids, h.ids...)
		}
		repeated := make([]float32, 0, n*length*size)
		mask := make([]int64, n*length)
		for i := range mask {
			mask[i] = 1
		}
		for range n {
			repeated = append(repeated, states...)
		}
		input, err := ort.NewTensor(ort.NewShape(n, t), ids)
		if err != nil {
			return nil, err
		}
		encoderStates, err := ort.NewTensor(ort.NewShape(n, length, size), repeated)
		if err != nil {
			input.Destroy()
			return nil, err
		}
		maskValue, err := ort.NewTensor(ort.NewShape(n, length), mask)
		if err != nil {
			input.Destroy()
			encoderStates.Destroy()
			return nil, err
		}
		results, err := decoder.call(map[string]ort.Value{"input_ids": input, "encoder_hidden_states": encoderStates, "encoder_attention_mask": maskValue})
		input.Destroy()
		encoderStates.Destroy()
		maskValue.Destroy()
		if err != nil {
			return nil, err
		}
		logits, ok := results["logits"].(*ort.Tensor[float32])
		if !ok {
			destroyAll(results)
			return nil, errors.New("the decoder has no float32 logits")
		}
		data := logits.GetData()
		vocab := int64(logits.GetShape()[2])
		type candidate struct {
			from  int
			token int64
			score float64
		}
		var candidates []candidate
		for i, h := range live {
			row := data[(int64(i)*t+t-1)*vocab : (int64(i)*t+t)*vocab]
			scores := choose(row)
			order := make([]int, len(scores))
			for j := range order {
				order[j] = j
			}
			sort.Slice(order, func(a, b int) bool { return scores[order[a]] > scores[order[b]] })
			for _, j := range order[:min(width, len(order))] {
				if math.IsInf(float64(scores[j]), -1) {
					break
				}
				candidates = append(candidates, candidate{i, int64(j), h.score + float64(scores[j])})
			}
		}
		destroyAll(results)
		sort.Slice(candidates, func(a, b int) bool { return candidates[a].score > candidates[b].score })
		var next []hypothesis
		for _, c := range candidates {
			if len(next) >= width {
				break
			}
			if c.token == req.End {
				done = append(done, hypothesis{ids: slices.Clone(live[c.from].ids[1:]), score: c.score})
				continue
			}
			ids := append(slices.Clone(live[c.from].ids), c.token)
			next = append(next, hypothesis{ids: ids, score: c.score})
		}
		live = next
		if len(done) >= width {
			break
		}
	}
	for _, h := range live {
		done = append(done, hypothesis{ids: h.ids[1:], score: h.score})
	}
	normal := func(h hypothesis) float64 { return h.score / float64(len(h.ids)+1) }
	sort.SliceStable(done, func(a, b int) bool { return normal(done[a]) > normal(done[b]) })
	var answer []onnxwire.Sequence
	seen := map[string]bool{}
	for _, h := range done {
		key := fmt.Sprint(h.ids)
		if seen[key] {
			continue
		}
		seen[key] = true
		answer = append(answer, onnxwire.Sequence{Score: float32(normal(h)), IDs: h.ids})
		if len(answer) == best {
			break
		}
	}
	return answer, nil
}
