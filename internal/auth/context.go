package auth

import "context"

// subjectKey 是上下文中存储 Subject 的键类型。
type subjectKey struct{}

// WithSubject 将经过身份验证的主体信息存储到上下文中。
func WithSubject(ctx context.Context, subject *Subject) context.Context {
	if subject == nil {
		return ctx
	}
	subject.normalise()
	return context.WithValue(ctx, subjectKey{}, subject)
}

// SubjectFromContext 从上下文中提取经过身份验证的主体信息。
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
