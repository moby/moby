package beam

import (
	"fmt"
	"log"
	"strings"
)

func Hub() Stream {
	inside, outside := Pipe()
	go func() {
		defer fmt.Printf("hub stopped\n")
		handlers := make(map[string]Stream)
		for {
			msg, err := inside.Receive()
			if err != nil {
				return
			}
			if msg.Stream == nil {
				continue
			}
			// FIXME: use proper word parsing
			words := strings.Split(string(msg.Data), " ")
			if words[0] == "register" {
				if len(words) < 2 {
					msg.Stream.Send(Message{Data: []byte("Usage: register COMMAND\n")})
					msg.Stream.Close()
				}
				for _, cmd := range words[1:] {
					fmt.Printf("Registering handler for %s\n", cmd)
					handlers[cmd] = msg.Stream
				}
				msg.Stream.Send(Message{Data: []byte("test on registered handler\n")})
			} else if words[0] == "commands" {
				JobHandler(func(job *Job) {
					fmt.Fprintf(job.Stdout, "%d commands:\n", len(handlers))
					for cmd := range handlers {
						fmt.Fprintf(job.Stdout, "%s\n", cmd)
					}
				}).Send(msg)
			} else if handler, exists := handlers[words[0]]; exists {
				err := handler.Send(msg)
				if err != nil {
					log.Printf("Error sending to %s handler: %s. De-registering handler.\n", words[0], err)
					delete(handlers, words[0])
				}
			} else {
				msg.Stream.Send(Message{Data: []byte("No such command: " + words[0] + "\n")})
			}
		}
	}()
	return outside
}
