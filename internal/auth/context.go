package auth

import "context"

type subjectKey struct{}

// WithSubject attaches the authenticated subject to the provided context.
func WithSubject(ctx context.Context, subject *Subject) context.Context {
	if subject == nil {
		return ctx
	}
	subject.normalise()
	return context.WithValue(ctx, subjectKey{}, subject)
}

// SubjectFromContext extracts the authenticated subject from the context, if
// present.
func SubjectFromContext(ctx context.Context) *Subject {
	if ctx == nil {
		return nil
	}
	if subject, ok := ctx.Value(subjectKey{}).(*Subject); ok {
		subject.normalise()
		return subject
	}
	return nil
}
