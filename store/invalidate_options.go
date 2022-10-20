package store

// InvalidateOption represents a cache invalidation function.
type InvalidateOption func(o *invalidateOptions)

type invalidateOptions struct {
	tags []string
}

func (o *invalidateOptions) isEmpty() bool {
	return len(o.tags) == 0
}

func (o *invalidateOptions) Tags() []string {
	return o.tags
}

func applyInvalidateOptionsWithDefault(defaultOptions *invalidateOptions, opts ...InvalidateOption) *invalidateOptions {
	returnedOptions := ApplyInvalidateOptions(opts...)

	if returnedOptions == new(invalidateOptions) {
		returnedOptions = defaultOptions
	}

	return returnedOptions
}

func ApplyInvalidateOptions(opts ...InvalidateOption) *invalidateOptions {
	o := &invalidateOptions{}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

// WithInvalidateTags allows setting the invalidate tags.
func WithInvalidateTags(tags []string) InvalidateOption {
	return func(o *invalidateOptions) {
		o.tags = tags
	}
}
