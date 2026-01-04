package herald

type Subscription struct {
	C      <-chan Message
	cancel func()
}
