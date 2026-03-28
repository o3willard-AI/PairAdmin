package filter

// Filter applies a transformation to content.
type Filter interface {
	Apply(content string) (string, error)
}

// Pipeline runs filters in sequence. First filter's output is second filter's input.
// If any filter returns an error, the pipeline returns the error and content processed so far.
type Pipeline struct {
	filters []Filter
}

// NewPipeline creates a Pipeline that applies the given filters in order.
func NewPipeline(filters ...Filter) *Pipeline {
	return &Pipeline{filters: filters}
}

// Apply runs the content through each filter in order.
// If any filter returns an error, the pipeline stops and returns the error with
// whatever content was produced up to that point.
func (p *Pipeline) Apply(content string) (string, error) {
	var err error
	for _, f := range p.filters {
		content, err = f.Apply(content)
		if err != nil {
			return content, err
		}
	}
	return content, nil
}
