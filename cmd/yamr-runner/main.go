package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"jinal--shah/yamr-run/internal/apprunner"
	"jinal--shah/yamr-run/internal/cli"
)

const (
	exitSuccess = 0
	exitFailure = 1
)

// make my testing life easier, just inject the mock functions needed for tests
// Don't need an interface, just a function type and we'll pass the functions from
// the Public api func to the private one. e.g. from Run() to run()
type appRunnerFunc func(
	context.Context,
	apprunner.Options,
) (apprunner.Result, error)

type cliParserFunc func(
	[]string,
	[]string,
) (cli.Options, error)

type getWdFunc func() (string, error)

func handleHelp(args []string) (bool, error) {
	hadHelp := false
	help, err := cli.ParseHelp(
		args,
	)
	if err != nil {
		fmt.Fprintln(
			os.Stderr,
			err,
		)

		return hadHelp, err
	}

	if help.Requested {
		hadHelp = true
		output, err := cli.Help(
			help.Topic,
		)
		if err != nil {
			fmt.Fprintln(
				os.Stderr,
				err,
			)
		} else {

			fmt.Fprint(
				os.Stdout,
				output,
			)
		}
		return hadHelp, err
	}

	return hadHelp, nil

}

func run(
	ctx context.Context,
	args []string,
	environ []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	parseCLI cliParserFunc,
	runDirFunc getWdFunc,
	runApp appRunnerFunc,
) int {

	hadHelp, err := handleHelp(
		args,
	)

	if err != nil {
		return exitFailure
	}

	if hadHelp {
		return exitSuccess
	}

	options, err := parseCLI(
		args,
		environ,
	)
	if err != nil {
		fmt.Fprintf(
			stderr,
			"yamr-runner: %v\n",
			err,
		)

		return exitFailure
	}

	if err != nil {
		fmt.Fprintf(
			stderr,
			"yamr-runner: %v\n",
			err,
		)

		return exitFailure
	}

	runDir, err := runDirFunc()
	if err != nil {
		fmt.Fprintf(
			stderr,
			"yamr-runner: could not get get current working directory: %v\n",
			err,
		)

		return exitFailure
	}

	result, err := runApp(
		ctx,
		apprunner.Options{
			CLI:     options,
			RunDir:  runDir,
			HostEnv: environ,
			Stdin:   stdin,
			Stdout:  stdout,
			Stderr:  stderr,
		},
	)
	if err != nil {
		fmt.Fprintf(
			stderr,
			"yamr-runner: %v\n",
			err,
		)

		return exitFailure
	}

	if result.Declined {
		return exitSuccess
	}

	if result.Execution.Failed() {
		return exitFailure
	}

	return exitSuccess
}

// main() should just deal with signals
// and invoking the one true entrypoint (run())
func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	exitCode := run(
		ctx,
		os.Args[1:],
		os.Environ(),
		os.Stdin,
		os.Stdout,
		os.Stderr,
		cli.Parse,
		os.Getwd,
		apprunner.Run,
	)

	os.Exit(exitCode)
}
