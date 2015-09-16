package cli

import (
	"os"
)

func Example() {
	app := NewApp()
	app.Name = "todo"
	app.Usage = "task list on the command line"
	app.Commands = []Command{
		{
			Name:    "add",
			Aliases: []string{"a"},
			Usage:   "add a task to the list",
			Action: func(c *Context) {
				println("added task: ", c.Args().First())
			},
		},
		{
			Name:    "complete",
			Aliases: []string{"c"},
			Usage:   "complete a task on the list",
			Action: func(c *Context) {
				println("completed task: ", c.Args().First())
			},
		},
	}

	app.Run(os.Args)
}

func ExampleSubcommand() {
	app := NewApp()
	app.Name = "say"
	app.Commands = []Command{
		{
			Name:        "hello",
			Aliases:     []string{"hi"},
			Usage:       "use it to see a description",
			Description: "This is how we describe hello the function",
			Subcommands: []Command{
				{
					Name:        "english",
					Aliases:     []string{"en"},
					Usage:       "sends a greeting in english",
					Description: "greets someone in english",
					Flags: []Flag{
						StringFlag{
							Name:  "name",
							Value: "Bob",
							Usage: "Name of the person to greet",
						},
					},
					Action: func(c *Context) {
						println("Hello, ", c.String("name"))
					},
				}, {
					Name:    "spanish",
					Aliases: []string{"sp"},
					Usage:   "sends a greeting in spanish",
					Flags: []Flag{
						StringFlag{
							Name:  "surname",
							Value: "Jones",
							Usage: "Surname of the person to greet",
						},
					},
					Action: func(c *Context) {
						println("Hola, ", c.String("surname"))
					},
				}, {
					Name:    "french",
					Aliases: []string{"fr"},
					Usage:   "sends a greeting in french",
					Flags: []Flag{
						StringFlag{
							Name:  "nickname",
							Value: "Stevie",
							Usage: "Nickname of the person to greet",
						},
					},
					Action: func(c *Context) {
						println("Bonjour, ", c.String("nickname"))
					},
				},
			},
		}, {
			Name:  "bye",
			Usage: "says goodbye",
			Action: func(c *Context) {
				println("bye")
			},
		},
	}

	app.Run(os.Args)
}
