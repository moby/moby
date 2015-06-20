// go:generate git tag | tail -1
package main

import (
	"compress/gzip"
	"io"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/vbatts/tar-split/tar/asm"
	"github.com/vbatts/tar-split/tar/storage"
)

func main() {
	app := cli.NewApp()
	app.Name = "tar-split"
	app.Usage = "tar assembly and disassembly utility"
	app.Version = "0.9.2"
	app.Author = "Vincent Batts"
	app.Email = "vbatts@hashbangbash.com"
	app.Action = cli.ShowAppHelp
	app.Before = func(c *cli.Context) error {
		logrus.SetOutput(os.Stderr)
		if c.Bool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug, D",
			Usage: "debug output",
			// defaults to false
		},
	}
	app.Commands = []cli.Command{
		{
			Name:    "disasm",
			Aliases: []string{"d"},
			Usage:   "disassemble the input tar stream",
			Action:  CommandDisasm,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "output",
					Value: "tar-data.json.gz",
					Usage: "output of disassembled tar stream",
				},
			},
		},
		{
			Name:    "asm",
			Aliases: []string{"a"},
			Usage:   "assemble tar stream",
			Action:  CommandAsm,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "input",
					Value: "tar-data.json.gz",
					Usage: "input of disassembled tar stream",
				},
				cli.StringFlag{
					Name:  "output",
					Value: "-",
					Usage: "reassembled tar archive",
				},
				cli.StringFlag{
					Name:  "path",
					Value: "",
					Usage: "relative path of extracted tar",
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func CommandDisasm(c *cli.Context) {
	if len(c.Args()) != 1 {
		logrus.Fatalf("please specify tar to be disabled <NAME|->")
	}
	if len(c.String("output")) == 0 {
		logrus.Fatalf("--output filename must be set")
	}

	// Set up the tar input stream
	var inputStream io.Reader
	if c.Args()[0] == "-" {
		inputStream = os.Stdin
	} else {
		fh, err := os.Open(c.Args()[0])
		if err != nil {
			logrus.Fatal(err)
		}
		defer fh.Close()
		inputStream = fh
	}

	// Set up the metadata storage
	mf, err := os.OpenFile(c.String("output"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(0600))
	if err != nil {
		logrus.Fatal(err)
	}
	defer mf.Close()
	mfz := gzip.NewWriter(mf)
	defer mfz.Close()
	metaPacker := storage.NewJSONPacker(mfz)

	// we're passing nil here for the file putter, because the ApplyDiff will
	// handle the extraction of the archive
	its, err := asm.NewInputTarStream(inputStream, metaPacker, nil)
	if err != nil {
		logrus.Fatal(err)
	}
	i, err := io.Copy(os.Stdout, its)
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("created %s from %s (read %d bytes)", c.String("output"), c.Args()[0], i)
}

func CommandAsm(c *cli.Context) {
	if len(c.Args()) > 0 {
		logrus.Warnf("%d additional arguments passed are ignored", len(c.Args()))
	}
	if len(c.String("input")) == 0 {
		logrus.Fatalf("--input filename must be set")
	}
	if len(c.String("output")) == 0 {
		logrus.Fatalf("--output filename must be set ([FILENAME|-])")
	}
	if len(c.String("path")) == 0 {
		logrus.Fatalf("--path must be set")
	}

	var outputStream io.Writer
	if c.String("output") == "-" {
		outputStream = os.Stdout
	} else {
		fh, err := os.Create(c.String("output"))
		if err != nil {
			logrus.Fatal(err)
		}
		defer fh.Close()
		outputStream = fh
	}

	// Get the tar metadata reader
	mf, err := os.Open(c.String("input"))
	if err != nil {
		logrus.Fatal(err)
	}
	defer mf.Close()
	mfz, err := gzip.NewReader(mf)
	if err != nil {
		logrus.Fatal(err)
	}
	defer mfz.Close()

	metaUnpacker := storage.NewJSONUnpacker(mfz)
	// XXX maybe get the absolute path here
	fileGetter := storage.NewPathFileGetter(c.String("path"))

	ots := asm.NewOutputTarStream(fileGetter, metaUnpacker)
	defer ots.Close()
	i, err := io.Copy(outputStream, ots)
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("created %s from %s and %s (wrote %d bytes)", c.String("output"), c.String("path"), c.String("input"), i)
}
