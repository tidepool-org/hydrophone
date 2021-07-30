package clients

type Notifier interface {
	Send(addresses []string, subject, content string, tags map[string]string) (int, string)
}
