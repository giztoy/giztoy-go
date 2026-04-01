package match

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"strconv"
	"strings"
	"text/template"

	"github.com/giztoy/giztoy-go/pkg/genx"
	"github.com/giztoy/giztoy-go/pkg/genx/generators"

	_ "embed"
)

//go:embed default.gotmpl
var defaultPromptTpl string

type Option func(*compileConfig)

type compileConfig struct {
	tpl string
}

// WithTpl sets a custom prompt template.
func WithTpl(tpl string) Option {
	return func(c *compileConfig) {
		c.tpl = tpl
	}
}

// Matcher is a compiled matcher built from rules.
type Matcher struct {
	systemPrompt string
	specs        map[string]map[string]Var
}

// SystemPrompt returns the rendered system prompt for debugging.
func (m *Matcher) SystemPrompt() string {
	return m.systemPrompt
}

// Arg holds a matched argument's value along with its definition.
type Arg struct {
	Value    any
	Var      Var
	HasValue bool
}

// Result is the structured output from a single match.
type Result struct {
	Rule    string
	Args    map[string]Arg
	RawText string
}

type MatchOption func(*matchConfig)

type matchConfig struct {
	gen genx.Generator
}

// WithGenerator sets a custom generator for Match.
func WithGenerator(gen genx.Generator) MatchOption {
	return func(c *matchConfig) {
		c.gen = gen
	}
}

// Match executes the matcher against user input and returns streaming results.
func (m *Matcher) Match(ctx context.Context, pattern string, mc genx.ModelContext, opts ...MatchOption) iter.Seq2[Result, error] {
	cfg := &matchConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	mcb := &genx.ModelContextBuilder{}
	mcb.PromptText("", m.systemPrompt)
	internal := mcb.Build()

	combined := genx.ModelContexts(mc, internal)

	return func(yield func(Result, error) bool) {
		var stream genx.Stream
		var err error

		if cfg.gen != nil {
			stream, err = cfg.gen.GenerateStream(ctx, pattern, combined)
		} else {
			stream, err = generators.GenerateStream(ctx, pattern, combined)
		}
		if err != nil {
			yield(Result{}, fmt.Errorf("generate: %w", err))
			return
		}
		defer stream.Close()

		var sb strings.Builder
		pending := ""
		stopped := false

		flush := func(line string) bool {
			if stopped {
				return false
			}
			line = strings.TrimSpace(line)
			if line == "" {
				return true
			}
			r, ok := m.parseLine(line)
			if ok {
				if !yield(r, nil) {
					stopped = true
					return false
				}
			}
			return true
		}

		for {
			chunk, err := stream.Next()
			if err != nil {
				if !errors.Is(err, genx.ErrDone) && !errors.Is(err, io.EOF) && !stopped {
					if !yield(Result{}, err) {
						stopped = true
					}
				}
				break
			}
			if chunk != nil && chunk.Part != nil {
				if text, ok := chunk.Part.(genx.Text); ok {
					sb.WriteString(string(text))
				}
			}

			s := pending + sb.String()
			sb.Reset()

			for {
				i := strings.IndexByte(s, '\n')
				if i < 0 {
					pending = s
					break
				}
				line := s[:i]
				s = s[i+1:]
				if !flush(line) {
					return
				}
			}
		}

		if !stopped {
			flush(pending)
		}
	}
}

func (m *Matcher) parseLine(line string) (Result, bool) {
	name, kv, hasColon := strings.Cut(line, ":")
	name = strings.TrimSpace(name)

	if name == "" {
		return Result{RawText: line}, true
	}

	vars, ok := m.specs[name]
	if !ok {
		return Result{RawText: line}, true
	}

	var args map[string]Arg
	if hasColon {
		args = m.parseKVToArgs(strings.TrimSpace(kv), vars)
	} else {
		args = m.parseKVToArgs("", vars)
	}
	return Result{Rule: name, Args: args}, true
}

func (m *Matcher) parseKVToArgs(kv string, vars map[string]Var) map[string]Arg {
	args := make(map[string]Arg)

	for name, v := range vars {
		args[name] = Arg{Value: nil, Var: v, HasValue: false}
	}

	if strings.TrimSpace(kv) == "" {
		return args
	}

	for part := range strings.SplitSeq(kv, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" {
			continue
		}

		varDef, exists := vars[k]
		if !exists {
			continue
		}

		var typedValue any = v
		switch varDef.Type {
		case "int":
			if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
				typedValue = parsed
			}
		case "float":
			if parsed, err := strconv.ParseFloat(v, 64); err == nil {
				typedValue = parsed
			}
		case "bool":
			if parsed, err := strconv.ParseBool(v); err == nil {
				typedValue = parsed
			}
		}

		args[k] = Arg{Value: typedValue, Var: varDef, HasValue: true}
	}

	return args
}

// Collect consumes a streaming sequence into a slice.
func Collect(seq iter.Seq2[Result, error]) ([]Result, error) {
	var out []Result
	for r, err := range seq {
		if err != nil {
			return out, err
		}
		out = append(out, r)
	}
	return out, nil
}

// Compile compiles rules into a reusable Matcher.
func Compile(rules []*Rule, opts ...Option) (*Matcher, error) {
	cfg := &compileConfig{tpl: defaultPromptTpl}
	for _, opt := range opts {
		opt(cfg)
	}

	data, err := buildPromptData(rules)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New("prompt").Funcs(template.FuncMap{
		"inc": func(i int) int { return i + 1 },
	}).Parse(cfg.tpl)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	specs := make(map[string]map[string]Var, len(rules))
	for _, r := range rules {
		if r == nil {
			continue
		}
		if _, exists := specs[r.Name]; exists {
			slog.Warn("match: duplicate rule name, skipping", "name", r.Name)
			continue
		}
		specs[r.Name] = r.Vars
	}

	return &Matcher{systemPrompt: buf.String(), specs: specs}, nil
}

type promptData struct {
	References map[string]string
	Rules      []ruleData
}

type ruleData struct {
	Name     string
	Patterns []patternData
	Examples []Example
}

type patternData struct {
	Input  string
	Output string
}

func buildPromptData(rules []*Rule) (*promptData, error) {
	data := &promptData{References: make(map[string]string)}
	for _, r := range rules {
		if r != nil {
			if err := r.compileTo(data); err != nil {
				return nil, err
			}
		}
	}
	return data, nil
}
