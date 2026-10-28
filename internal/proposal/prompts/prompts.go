// Package prompts holds the LLM prompt packages of Proposal (D-059, D-060, D-069): plain text files
// embedded with go:embed, each with a small manifest, a version identity of name@semver plus a
// sha256 of the request prefix, and a committed lock file that a test checks.
//
// A package is everything the API receives before the user message: the system text, the tool
// definitions and the structured output schema, plus the model and effort. Call-specific data
// (brief, context notes, language) never belongs in a package; it goes into the user message as a
// JSON block, so the prefix stays byte-identical and cacheable.
package prompts

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/furknozg/coldbru/internal/proposal/catalog"
)

//go:embed screen segment
var files embed.FS

// Names of the packages shipped here.
const (
	NameScreen  = "screen"
	NameSegment = "segment"
)

// CatalogPart is the manifest entry that stands for the rendered filter catalog.
const CatalogPart = "@catalog"

// semver is the shape of Manifest.Version (MAJOR.MINOR.PATCH, D-059).
var semver = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// Efforts the manifest may name (output_config.effort).
var efforts = map[string]bool{"low": true, "medium": true, "high": true, "xhigh": true, "max": true}

// Manifest is the manifest.yaml of a package directory.
type Manifest struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Model   string `yaml:"model"`
	Effort  string `yaml:"effort"`
	// RequiresDSL is the DSL version the prompt teaches (D-045); a test compares it with dsl.Version.
	RequiresDSL string `yaml:"requires_dsl"`
	// System lists the text files joined, in this order, into the system text. CatalogPart
	// stands for the rendered catalog.
	System []string `yaml:"system"`
	// Tools is the file with the tool definitions, a JSON array; empty for none.
	Tools string `yaml:"tools"`
	// OutputSchema is the file with the structured output JSON schema; empty for none.
	OutputSchema string `yaml:"output_schema"`
}

// Tool is one tool definition in canonical JSON, exactly as it is sent.
type Tool struct {
	Name string
	JSON json.RawMessage
}

// Package is a loaded prompt package. Treat it as read-only.
type Package struct {
	Name        string
	Version     string
	Model       string
	Effort      string
	RequiresDSL string
	// System is the complete system text.
	System string
	// Tools are the tool definitions in canonical JSON (sorted keys, compact), in manifest order.
	Tools []Tool
	// OutputSchema is the output_config.format schema in canonical JSON; nil when the package has none.
	OutputSchema json.RawMessage
	// SHA256 is the hex digest of Prefix (D-059).
	SHA256 string
}

// ID returns "name@version".
func (p *Package) ID() string { return p.Name + "@" + p.Version }

// prefix is what the hash covers (D-059): the parts of the request that stay the same between calls.
type prefix struct {
	System       string            `json:"system"`
	Tools        []json.RawMessage `json:"tools"`
	OutputFormat json.RawMessage   `json:"output_format,omitempty"`
	Model        string            `json:"model"`
	Effort       string            `json:"effort"`
}

// Prefix returns the canonical bytes the hash is computed over: system, tools, output schema,
// model and effort as JSON with sorted keys. The user message is not part of it.
func (p *Package) Prefix() []byte {
	pre := prefix{System: p.System, Tools: make([]json.RawMessage, len(p.Tools)), Model: p.Model, Effort: p.Effort}
	for i, t := range p.Tools {
		pre.Tools[i] = t.JSON
	}
	pre.OutputFormat = p.OutputSchema
	b, err := marshal(pre)
	if err != nil {
		// Every field is a string or already valid JSON, checked by Build.
		panic("prompts: marshal prefix: " + err.Error())
	}
	return b
}

// Build validates the manifest and assembles a Package from its parts. system is the final system
// text, tools a JSON array of tool definitions (or empty) and outputSchema a JSON schema (or empty).
// Tools and schema are stored in canonical form, so the bytes sent equal the bytes hashed.
func Build(m Manifest, system string, tools, outputSchema []byte) (*Package, error) {
	if strings.TrimSpace(m.Name) == "" {
		return nil, errors.New("prompts: manifest has no name")
	}
	if !semver.MatchString(m.Version) {
		return nil, fmt.Errorf("prompts: %s: version %q is not MAJOR.MINOR.PATCH", m.Name, m.Version)
	}
	if strings.TrimSpace(m.Model) == "" {
		return nil, fmt.Errorf("prompts: %s: manifest has no model", m.Name)
	}
	if !efforts[m.Effort] {
		return nil, fmt.Errorf("prompts: %s: effort %q is not one of low, medium, high, xhigh, max", m.Name, m.Effort)
	}
	if strings.TrimSpace(system) == "" {
		return nil, fmt.Errorf("prompts: %s: empty system text", m.Name)
	}
	p := &Package{
		Name: m.Name, Version: m.Version, Model: m.Model, Effort: m.Effort,
		RequiresDSL: m.RequiresDSL, System: system,
	}
	if len(bytes.TrimSpace(tools)) > 0 {
		var defs []json.RawMessage
		if err := json.Unmarshal(tools, &defs); err != nil {
			return nil, fmt.Errorf("prompts: %s: tools must be a JSON array: %w", m.Name, err)
		}
		seen := map[string]bool{}
		for i, d := range defs {
			c, err := Canonical(d)
			if err != nil {
				return nil, fmt.Errorf("prompts: %s: tool %d: %w", m.Name, i, err)
			}
			var head struct {
				Name        string          `json:"name"`
				InputSchema json.RawMessage `json:"input_schema"`
			}
			if err := json.Unmarshal(c, &head); err != nil || head.Name == "" || len(head.InputSchema) == 0 {
				return nil, fmt.Errorf("prompts: %s: tool %d needs a name and an input_schema", m.Name, i)
			}
			if seen[head.Name] {
				return nil, fmt.Errorf("prompts: %s: duplicate tool %q", m.Name, head.Name)
			}
			seen[head.Name] = true
			p.Tools = append(p.Tools, Tool{Name: head.Name, JSON: c})
		}
	}
	if len(bytes.TrimSpace(outputSchema)) > 0 {
		c, err := Canonical(outputSchema)
		if err != nil {
			return nil, fmt.Errorf("prompts: %s: output schema: %w", m.Name, err)
		}
		var obj map[string]any
		if err := json.Unmarshal(c, &obj); err != nil || obj["type"] != "object" {
			return nil, fmt.Errorf("prompts: %s: output schema must be a JSON object schema", m.Name)
		}
		p.OutputSchema = c
	}
	sum := sha256.Sum256(p.Prefix())
	p.SHA256 = hex.EncodeToString(sum[:])
	return p, nil
}

// Canonical re-encodes JSON with sorted object keys, no insignificant whitespace and no HTML
// escaping, keeping numbers as written. The same value always yields the same bytes.
func Canonical(raw []byte) (json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if dec.More() {
		return nil, errors.New("invalid JSON: trailing data")
	}
	return marshal(v)
}

// marshal encodes v without HTML escaping and without the trailing newline.
func marshal(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// load reads the package in dir of fsys. root is the catalog rendered for CatalogPart.
func load(fsys fs.FS, dir string, root *catalog.Entity) (*Package, error) {
	raw, err := fs.ReadFile(fsys, path.Join(dir, "manifest.yaml"))
	if err != nil {
		return nil, err
	}
	var m Manifest
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("prompts: %s/manifest.yaml: %w", dir, err)
	}
	if m.Name != dir {
		return nil, fmt.Errorf("prompts: %s/manifest.yaml names the package %q", dir, m.Name)
	}
	parts := make([]string, 0, len(m.System))
	for _, f := range m.System {
		if f == CatalogPart {
			parts = append(parts, RenderCatalog(root))
			continue
		}
		b, err := fs.ReadFile(fsys, path.Join(dir, f))
		if err != nil {
			return nil, fmt.Errorf("prompts: %s: system part: %w", dir, err)
		}
		parts = append(parts, strings.TrimSpace(string(b)))
	}
	var tools, schema []byte
	if m.Tools != "" {
		if tools, err = fs.ReadFile(fsys, path.Join(dir, m.Tools)); err != nil {
			return nil, fmt.Errorf("prompts: %s: tools: %w", dir, err)
		}
	}
	if m.OutputSchema != "" {
		if schema, err = fs.ReadFile(fsys, path.Join(dir, m.OutputSchema)); err != nil {
			return nil, fmt.Errorf("prompts: %s: output schema: %w", dir, err)
		}
	}
	return Build(m, strings.Join(parts, "\n\n"), tools, schema)
}

// loaded are the shipped packages, assembled once at startup. A broken embed is a build defect
// that the tests catch, so a failure panics.
var loaded = func() map[string]*Package {
	out := map[string]*Package{}
	for _, name := range []string{NameScreen, NameSegment} {
		p, err := load(files, name, catalog.Root)
		if err != nil {
			panic(err)
		}
		out[name] = p
	}
	return out
}()

// Screen returns the injection pre-screen package (Haiku 5.5, D-026, D-069).
func Screen() *Package { return loaded[NameScreen] }

// Segment returns the segment tool-loop package (Sonnet 5.5, D-026, D-069).
func Segment() *Package { return loaded[NameSegment] }

// All returns every shipped package ordered by name.
func All() []*Package {
	out := make([]*Package, 0, len(loaded))
	for _, p := range loaded {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
