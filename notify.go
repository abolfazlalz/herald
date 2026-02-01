package herald

type HandlerFunc func(*MessageContext, *Message)
