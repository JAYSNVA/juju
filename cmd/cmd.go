package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"launchpad.net/gnuflag"
	"launchpad.net/juju-core/log"
	"os"
	"path/filepath"
	"strings"
)

// ErrSilent can be returned from Run to signal that Main should exit with
// code 1 without producing error output.
var ErrSilent = errors.New("cmd: error out silently")

// Command is implemented by types that interpret command-line arguments.
type Command interface {
	// Info returns information about the Command.
	Info() *Info

	// SetFlags adds command specific flags to the flag set.
	SetFlags(f *gnuflag.FlagSet)

	// Init initializes the Command before running.
	Init(args []string) error

	// Run will execute the Command as directed by the options and positional
	// arguments passed to Init.
	Run(ctx *Context) error
}

// CommandBase provides the default implementation for SetFlags, Init, and Help.
type CommandBase struct{}

// SetFlags does nothing in the simplest case.
func (c *CommandBase) SetFlags(f *gnuflag.FlagSet) {}

// Init in the simplest case makes sure there are no args.
func (c *CommandBase) Init(args []string) error {
	return CheckEmpty(args)
}

// Context represents the run context of a Command. Command implementations
// should interpret file names relative to Dir (see AbsPath below), and print
// output and errors to Stdout and Stderr respectively.
type Context struct {
	Dir    string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// AbsPath returns an absolute representation of path, with relative paths
// interpreted as relative to ctx.Dir.
func (ctx *Context) AbsPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(ctx.Dir, path)
}

// Info holds some of the usage documentation of a Command.
type Info struct {
	// Name is the Command's name.
	Name string

	// Args describes the command's expected positional arguments.
	Args string

	// Purpose is a short explanation of the Command's purpose.
	Purpose string

	// Doc is the long documentation for the Command.
	Doc string

	// Aliases are other names for the Command.
	Aliases []string
}

// Help renders i's content, along with documentation for any
// flags defined in f. It calls f.SetOutput(ioutil.Discard).
func (i *Info) Help(f *gnuflag.FlagSet) []byte {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "usage: %s", i.Name)
	hasOptions := false
	f.VisitAll(func(f *gnuflag.Flag) { hasOptions = true })
	if hasOptions {
		fmt.Fprintf(buf, " [options]")
	}
	if i.Args != "" {
		fmt.Fprintf(buf, " %s", i.Args)
	}
	fmt.Fprintf(buf, "\n")
	if i.Purpose != "" {
		fmt.Fprintf(buf, "purpose: %s\n", i.Purpose)
	}
	if hasOptions {
		fmt.Fprintf(buf, "\noptions:\n")
		f.SetOutput(buf)
		f.PrintDefaults()
	}
	f.SetOutput(ioutil.Discard)
	if i.Doc != "" {
		fmt.Fprintf(buf, "\n%s\n", strings.TrimSpace(i.Doc))
	}
	if len(i.Aliases) > 0 {
		fmt.Fprintf(buf, "\naliases: %s\n", strings.Join(i.Aliases, ", "))
	}
	return buf.Bytes()
}

// ParseArgs encapsulate the parsing of the args so this function can be
// called from the testing module too.
func ParseArgs(c Command, f *gnuflag.FlagSet, args []string) error {
	// If the command is a SuperCommand, we want to parse the args with
	// allowIntersperse=false (i.e. the first parameter to Parse.  This will
	// mean that the args may contain other options that haven't been defined
	// yet, and that only options that relate to the SuperCommand itself can
	// come prior to the subcommand name.
	_, isSuperCommand := c.(*SuperCommand)
	return f.Parse(!isSuperCommand, args)
}

// Errors from commands can be either ErrHelp, which means "show the help" or
// some other error related to needed flags missing, or needed positional args
// missing, in which case we should print the error and return a non-zero
// return code.
func handleCommandError(c Command, ctx *Context, err error, f *gnuflag.FlagSet) (int, bool) {
	if err == gnuflag.ErrHelp {
		ctx.Stderr.Write(c.Info().Help(f))
		return 0, true
	}
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "error: %v\n", err)
		return 2, true
	}
	return 0, false
}

// Main runs the given Command in the supplied Context with the given
// arguments, which should not include the command name. It returns a code
// suitable for passing to os.Exit.
func Main(c Command, ctx *Context, args []string) int {
	f := gnuflag.NewFlagSet(c.Info().Name, gnuflag.ContinueOnError)
	f.SetOutput(ioutil.Discard)
	c.SetFlags(f)
	if rc, done := handleCommandError(c, ctx, ParseArgs(c, f, args), f); done {
		return rc
	}
	// Since SuperCommands can also return gnuflag.ErrHelp errors, we need to
	// handle both those types of errors as well as "real" errors.
	if rc, done := handleCommandError(c, ctx, c.Init(f.Args()), f); done {
		return rc
	}
	if err := c.Run(ctx); err != nil {
		if err != ErrSilent {
			log.Printf("%s command failed: %s\n", c.Info().Name, err)
			fmt.Fprintf(ctx.Stderr, "error: %v\n", err)
		}
		return 1
	}
	return 0
}

// DefaultContext returns a Context suitable for use in non-hosted situations.
func DefaultContext() *Context {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		panic(err)
	}
	return &Context{
		Dir:    abs,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// CheckEmpty is a utility function that returns an error if args is not empty.
func CheckEmpty(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("unrecognized args: %q", args)
	}
	return nil
}
