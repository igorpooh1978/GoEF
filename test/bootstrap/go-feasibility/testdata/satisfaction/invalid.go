package invalid

type stringWriter interface{ Write(string) }
type genericWriter struct{}

func (genericWriter) Write[T any](T) {}

var _ stringWriter = genericWriter{}
