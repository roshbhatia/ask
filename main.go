package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/roshbhatia/go-utils/completion"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/roshbhatia/ask/internal/config"
	"github.com/roshbhatia/ask/internal/pane"
	"github.com/roshbhatia/ask/internal/provider"
	"github.com/roshbhatia/ask/internal/schema"
	"github.com/roshbhatia/ask/internal/store"
	"github.com/roshbhatia/ask/internal/templates"
	"github.com/roshbhatia/ask/internal/ui"
)

const about = `Agents in your shell!

Anything on stdin goes to the agent along with the prompt, so a pipe is optional.

  %[1]s what does git rebase --onto do
  cat main.go | %[1]s summarise this file
  cat log.txt | %[1]s -p local-model --schema 'level:error|warn|info, message:string' -- classify this
  %[1]s --show-input | pbcopy

The prompt is the bare words after the flags, so the -- is only needed when a flag
comes first. Quote a prompt that holds shell metacharacters, as in
%[1]s -p local-model 'what does the | operator do', or the shell reads them before %[1]s does.

No agent is the default one. %[1]s takes the first of these that says which to run:
the -p flag, the name it was called by, $ASK_PROVIDER, then provider.default from
the settings. With none of them it opens a picker.

  %[1]s --set-config provider.default=local-model
  %[1]s --get-config provider.default
  %[1]s --list-config

The _ and _j wrappers use whichever provider $ASK_PROVIDER or the settings name.
The trailing j asks for JSON. %[1]s --list-config prints the selected provider
and where that choice came from.

Providers answer --json and --schema in the shape asked for. The answer is
checked against that shape here, so a run that answers outside it is a failure
rather than a wrongly shaped success.

An agent given a shape may answer with one question instead, when guessing would
cost more than asking. With a terminal it asks and waits, up to three times. With
no terminal it prints the question and exits 3, so a script can tell a question
from a failure.

Press tab after --schema and the shell writes the spec for you: a whole example
when nothing is typed, and the list of types after a name and a colon.

  %[1]s --schema <tab>
  %[1]s --schema 'files:<tab>

--last sends what the previous command printed, so a pipe is not needed to hand
an agent an error you have just read.

  cargo build; %[1]s --last why did this fail
  %[1]s --show-last | head

Prompt templates live in the config directory and use {{variable}} placeholders.
A prompt template can name its default schema template.

  %[1]s schema save review-result 'summary:string, risks:[]string'
  %[1]s prompt save review --schema review-result --variable repo --variable focus=correctness
  %[1]s --template review --var repo=ask`

type options struct {
	prompt         string
	json           bool
	spec           string
	model          string
	provider       string
	replay         bool
	last           bool
	quiet          bool
	timeout        time.Duration
	template       string
	vars           []string
	schemaTemplate string

	showInput  bool
	showPrompt bool
	showOutput bool
	showLast   bool
	capture    bool

	setConfig  string
	getConfig  string
	listConfig bool
}

func (o options) show() string {
	switch {
	case o.showInput:
		return "input"
	case o.showPrompt:
		return "prompt"
	case o.showOutput:
		return "output"
	case o.showLast:
		return "last"
	}
	return ""
}

func called() string {
	return filepath.Base(os.Args[0])
}

// wrapper reads the name the binary was called by. A trailing j asks for JSON.
func wrapper(name string) (short string, asJSON bool, known bool, err error) {
	rest, is := strings.CutPrefix(name, "_")
	if !is {
		return "", false, false, nil
	}
	if rest == "" {
		return "", false, true, nil
	}
	if _, found, lookupErr := provider.Lookup(rest); lookupErr != nil {
		return "", false, false, lookupErr
	} else if found {
		return rest, false, true, nil
	}
	if trimmed, cut := strings.CutSuffix(rest, "j"); cut {
		if trimmed == "" {
			return "", true, true, nil
		}
		if _, found, lookupErr := provider.Lookup(trimmed); lookupErr != nil {
			return "", false, false, lookupErr
		} else if found {
			return trimmed, true, true, nil
		}
	}
	return "", false, false, nil
}

func command(opts *options) *cobra.Command {
	name := called()

	cmd := &cobra.Command{
		Use:   name + " [flags] [prompt...]",
		Short: "Agents in your shell!",
		Long:  fmt.Sprintf(about, name),
		Args:  cobra.ArbitraryArgs,

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			opts.prompt = strings.Join(args, " ")
			return run(*opts)
		},

		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	flags := cmd.Flags()
	flags.SetInterspersed(false)
	flags.BoolVarP(&opts.json, "json", "j", false, "answer in JSON, shape unspecified")
	flags.StringVarP(&opts.spec, "schema", "s", "", "answer in JSON, in this shape: a field spec such as 'name:string, tags:[]string, count:int?', where a trailing question mark makes a field optional and a bar makes an enum, or @path to a JSON Schema file")
	flags.StringVarP(&opts.model, "model", "m", "", "which model to run; press tab for the ones this agent names")
	flags.StringVarP(&opts.provider, "provider", "p", "", "which installed provider to run")
	flags.BoolVar(&opts.replay, "replay", false, "rerun the last input, with this prompt or the last one")
	flags.BoolVarP(&opts.last, "last", "l", false, "send what the previous command printed, instead of stdin")
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "no progress output at all")
	flags.DurationVar(&opts.timeout, "timeout", 10*time.Minute, "give up after this long")
	flags.StringVarP(&opts.template, "template", "t", "", "use a named prompt template")
	flags.StringArrayVar(&opts.vars, "var", nil, "set one prompt template variable as NAME=VALUE; repeat as needed")
	flags.StringVar(&opts.schemaTemplate, "schema-template", "", "use a named schema template")
	flags.BoolVar(&opts.showInput, "show-input", false, "print the last input and exit")
	flags.BoolVar(&opts.showPrompt, "show-prompt", false, "print the last prompt and exit")
	flags.BoolVar(&opts.showOutput, "show-output", false, "print the last answer and exit")
	flags.BoolVar(&opts.showLast, "show-last", false, "print what --last would send and exit")
	// A flag, not a subcommand, as the root takes a bare prompt and a `capture`
	// subcommand would eat `ask capture the flag`.
	flags.BoolVar(&opts.capture, "capture", false, "snapshot the terminal and exit")
	_ = flags.MarkHidden("capture")
	flags.StringVar(&opts.setConfig, "set-config", "", "write one setting, as KEY=VALUE, and exit")
	flags.StringVar(&opts.getConfig, "get-config", "", "print one setting and exit")
	flags.BoolVar(&opts.listConfig, "list-config", false, "print every setting and exit")

	_ = cmd.RegisterFlagCompletionFunc("provider", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return agents(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("model", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return models(*opts), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("get-config", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return config.Keys(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("set-config", func(_ *cobra.Command, _ []string, typed string) ([]string, cobra.ShellCompDirective) {
		return pairs(typed), cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("schema", func(_ *cobra.Command, _ []string, typed string) ([]string, cobra.ShellCompDirective) {
		offer, paths := schema.Complete(typed)
		if paths {
			return nil, cobra.ShellCompDirectiveDefault
		}
		return offer, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("template", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return templateNames("prompt"), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("schema-template", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return templateNames("schema"), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("timeout", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"30s\ta quick question",
			"2m\tone file",
			"10m\tthe default",
			"30m\ta whole repository",
		}, cobra.ShellCompDirectiveNoFileComp
	})
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.AddCommand(completionCommand(cmd))
	cmd.AddCommand(generateCommand())
	cmd.AddCommand(promptCommand())
	cmd.AddCommand(providerCommand())
	cmd.AddCommand(schemaCommand())

	return cmd
}

func generateCommand() *cobra.Command {
	var root string
	var check bool
	cmd := &cobra.Command{
		Use:    "generate",
		Short:  "Generate committed schemas",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(*cobra.Command, []string) error {
			configSchema, err := config.Schema()
			if err != nil {
				return err
			}
			providerSchema, err := provider.Schema()
			if err != nil {
				return err
			}
			generatedFiles := map[string][]byte{
				filepath.Join(root, "schema", "config.schema.json"):   configSchema,
				filepath.Join(root, "schema", "provider.schema.json"): providerSchema,
			}
			wireSchemas, err := provider.WireSchemas()
			if err != nil {
				return err
			}
			for name, content := range wireSchemas {
				generatedFiles[filepath.Join(root, "schema", name)] = content
			}
			for path, content := range generatedFiles {
				if err := generated(path, content, check); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", ".", "repository root")
	cmd.Flags().BoolVar(&check, "check", false, "fail when a generated file differs")
	return cmd
}

func generated(path string, content []byte, check bool) error {
	if check {
		existing, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(existing, content) {
			return fmt.Errorf("%s is stale; run hack/generate.sh", path)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func providerCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "provider", Short: "Inspect external inference providers"}
	var listJSON bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List discovered providers",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			loaded, err := provider.Loaded()
			if err != nil {
				return err
			}
			if listJSON {
				return json.NewEncoder(os.Stdout).Encode(loaded)
			}
			for _, item := range loaded {
				fmt.Printf("%-12s %s\n", item.Manifest.Name, item.Manifest.Description)
				actions := make([]string, 0, len(item.Manifest.Actions))
				for name := range item.Manifest.Actions {
					actions = append(actions, name)
				}
				sort.Strings(actions)
				for _, name := range actions {
					action := item.Manifest.Actions[name]
					fmt.Printf("  %-22s %s\n", name, action.Description)
					invocation := append([]string(nil), item.Manifest.Command...)
					invocation = append(invocation, action.Argv...)
					fmt.Printf("    command: %s\n", strings.Join(invocation, " "))
				}
			}
			return nil
		},
	}
	list.Flags().BoolVar(&listJSON, "json", false, "print JSON")

	var validateJSON bool
	validate := &cobra.Command{
		Use:   "validate [NAME]",
		Short: "Validate provider manifests and dependencies",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			here, _ := os.Getwd()
			reports, err := provider.Validate(name, here)
			if err != nil {
				return err
			}
			if validateJSON {
				return json.NewEncoder(os.Stdout).Encode(reports)
			}
			failed := false
			for _, report := range reports {
				status := "ok"
				if !report.OK() {
					status, failed = "failed", true
				}
				fmt.Printf("%s  %s\n", report.Provider, status)
				for _, check := range report.Checks {
					fmt.Printf("  %-11s %-5s %s\n", check.Kind, check.Status, check.Message)
				}
			}
			if failed {
				return errors.New("provider validation failed")
			}
			return nil
		},
	}
	validate.Flags().BoolVar(&validateJSON, "json", false, "print JSON")
	cmd.AddCommand(list, validate)
	return cmd
}

func templateNames(kind string) []string {
	names, _ := templates.List(kind)
	return names
}

func promptCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "prompt", Short: "Manage prompt templates"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List prompt templates",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			for _, name := range templateNames("prompt") {
				fmt.Println(name)
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:               "show NAME",
		Short:             "Print a prompt template",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completePromptNames,
		RunE: func(_ *cobra.Command, args []string) error {
			prompt, err := templates.LoadPrompt(args[0])
			if err != nil {
				return err
			}
			return printTemplate(prompt)
		},
	})

	var description, schemaName string
	var variables []string
	save := &cobra.Command{
		Use:   "save NAME",
		Short: "Save the last prompt as a template",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			raw, err := store.Prompt()
			if err != nil {
				return fmt.Errorf("no saved prompt: %w", err)
			}
			declared, err := templates.ParseVariables(variables)
			if err != nil {
				return err
			}
			path, err := templates.SavePrompt(templates.Prompt{
				Name: args[0], Description: description, Prompt: string(raw), Schema: schemaName, Variables: declared,
			})
			if err != nil {
				return err
			}
			fmt.Println(path)
			return nil
		},
	}
	save.Flags().StringVar(&description, "description", "", "describe when to use this prompt")
	save.Flags().StringVar(&schemaName, "schema", "", "associate a default schema template")
	save.Flags().StringArrayVar(&variables, "variable", nil, "declare NAME or NAME=DEFAULT; repeat as needed")
	_ = save.RegisterFlagCompletionFunc("schema", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return templateNames("schema"), cobra.ShellCompDirectiveNoFileComp
	})
	cmd.AddCommand(save)
	return cmd
}

func schemaCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "schema", Short: "Manage schema templates"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List schema templates",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			for _, name := range templateNames("schema") {
				fmt.Println(name)
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:               "show NAME",
		Short:             "Print a schema template",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSchemaNames,
		RunE: func(_ *cobra.Command, args []string) error {
			shape, err := templates.LoadSchema(args[0])
			if err != nil {
				return err
			}
			return printTemplate(shape)
		},
	})

	var description string
	save := &cobra.Command{
		Use:   "save NAME SPEC",
		Short: "Save a field spec or JSON Schema file as a template",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			shape, err := schema.Resolve(args[1])
			if err != nil {
				return err
			}
			path, err := templates.SaveSchema(templates.Schema{Name: args[0], Description: description, Schema: shape})
			if err != nil {
				return err
			}
			fmt.Println(path)
			return nil
		},
	}
	save.Flags().StringVar(&description, "description", "", "describe the structured result")
	cmd.AddCommand(save)
	return cmd
}

func completePromptNames(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return templateNames("prompt"), cobra.ShellCompDirectiveNoFileComp
}

func completeSchemaNames(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return templateNames("schema"), cobra.ShellCompDirectiveNoFileComp
}

func printTemplate(value any) error {
	raw, err := templates.Encode(value)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(raw)
	return err
}

func completionCommand(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:       "completion bash|zsh|fish|nu",
		Short:     "Generate shell completion",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "nu"},
		RunE: func(_ *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(os.Stdout)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, true)
			case "nu":
				out, err := completion.Generate("nu", completionSpec(root))
				if err != nil {
					return err
				}
				fmt.Fprintln(os.Stdout, out)
				return nil
			default:
				return fmt.Errorf("completion requires bash, zsh, fish, or nu")
			}
		},
	}
}

func completionSpec(cmd *cobra.Command) completion.Command {
	spec := completion.Command{Name: cmd.Name(), Description: cmd.Short}
	addCompletionFlags(&spec, cmd)
	addCompletionCommands(&spec, "", cmd)
	return spec
}

func addCompletionFlags(spec *completion.Command, cmd *cobra.Command) {
	cmd.NonInheritedFlags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		spec.Flags = append(spec.Flags, completion.Flag{
			Name:        flag.Name,
			Short:       flag.Shorthand,
			Description: flag.Usage,
			Value:       flag.NoOptDefVal == "",
		})
	})
}

func addCompletionCommands(spec *completion.Command, parent string, cmd *cobra.Command) {
	for _, child := range cmd.Commands() {
		if child.Name() == "completion" || !child.IsAvailableCommand() {
			continue
		}
		name := strings.TrimSpace(parent + " " + child.Name())
		subcommand := completion.Command{Name: name, Description: child.Short}
		addCompletionFlags(&subcommand, child)
		spec.Subcommands = append(spec.Subcommands, subcommand)
		addCompletionCommands(spec, name, child)
	}
}

// models offers what the agent about to run accepts, read from that CLI's help.
func models(opts options) []string {
	named, _, err := source()
	if err != nil {
		return nil
	}
	if opts.provider != "" {
		named = opts.provider
	}
	one, found, err := provider.Lookup(named)
	if err != nil || !found {
		return nil
	}

	offer := one.Models()
	for at, name := range offer {
		offer[at] = name + "\t" + one.Blurb
	}
	return offer
}

// agents offers every discovered provider.
func agents() []string {
	known, err := provider.Known()
	if err != nil {
		return nil
	}
	offer := make([]string, 0, len(known))
	for _, one := range known {
		says := one.Blurb
		if !one.Ready() {
			says = one.Binary + " is not on PATH"
		}
		offer = append(offer, one.Name+"\t"+says)
	}
	return offer
}

// pairs completes --set-config on both halves: the keys first, then that key's values.
func pairs(typed string) []string {
	key, _, is := strings.Cut(typed, "=")
	if !is {
		offer := make([]string, 0, len(config.Keys()))
		for _, one := range config.Keys() {
			offer = append(offer, one+"=")
		}
		return offer
	}
	for _, setting := range config.Settings() {
		if setting.Key != key || setting.Values == nil {
			continue
		}
		offer := make([]string, 0)
		values, err := setting.Values()
		if err != nil {
			return nil
		}
		for _, value := range values {
			offer = append(offer, key+"="+value)
		}
		return offer
	}
	return nil
}

// source names the agent that will run and which of the four ways named it. The
// settings file is the last of the four, so --set-config can report a value that
// nothing reads; this is what lets the settings output say so.
func source() (named string, from string, err error) {
	short, _, _, err := wrapper(called())
	if err != nil {
		return "", "", err
	}
	if short != "" {
		return short, "the name it was called by", nil
	}
	if named := os.Getenv("ASK_PROVIDER"); named != "" {
		return named, "$ASK_PROVIDER", nil
	}
	if settled, err := config.Get(config.ProviderDefault); err == nil && settled != "" {
		return settled, config.ProviderDefault, nil
	} else if err != nil {
		return "", "", err
	}
	return "", "", nil
}

// chosen walks the ways to name an agent, most explicit first, and asks only when
// none of them says anything.
func chosen(opts options) (provider.Provider, error) {
	short, _, _, err := wrapper(called())
	if err != nil {
		return nil, err
	}
	for _, named := range []string{opts.provider, short, os.Getenv("ASK_PROVIDER")} {
		if named != "" {
			return provider.Find(named)
		}
	}

	settled, err := config.Get(config.ProviderDefault)
	if err != nil {
		return nil, err
	}
	if settled != "" {
		return provider.Find(settled)
	}

	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return nil, fmt.Errorf("say which agent to run with -p, set $ASK_PROVIDER, or set %s", config.ProviderDefault)
	}
	known, err := provider.Discover()
	if err != nil {
		return nil, err
	}
	picked, err := ui.Pick(known)
	if err != nil {
		return nil, err
	}
	return picked.New(), nil
}

func piped() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}

func printSaved(which string) error {
	var read func() ([]byte, error)
	switch which {
	case "input":
		read = store.Input
	case "prompt":
		read = store.Prompt
	case "output":
		read = store.Output
	case "last":
		read = pane.Last
	}
	saved, err := read()
	switch {
	case err != nil && which == "last":
		return err
	case err != nil:
		return fmt.Errorf("no saved %s: %w", which, err)
	}
	if _, err := os.Stdout.Write(saved); err != nil {
		return err
	}
	if len(saved) > 0 && saved[len(saved)-1] != '\n' {
		fmt.Println()
	}
	return nil
}

// settings answers true when the run was about the settings rather than an agent.
func settings(opts options) (bool, error) {
	switch {
	case opts.setConfig != "":
		key, value, err := config.Set(opts.setConfig)
		if err != nil {
			return true, err
		}
		if value == "" {
			fmt.Printf("%s is now unset\n", key)
			return true, nil
		}
		fmt.Printf("%s=%s\n", key, value)
		if outer := os.Getenv("ASK_PROVIDER"); key == config.ProviderDefault && outer != "" {
			fmt.Fprintf(os.Stderr, "note: $ASK_PROVIDER=%s outranks %s, so nothing reads this until that is unset\n", outer, key)
		}
		return true, nil
	case opts.getConfig != "":
		value, err := config.Get(opts.getConfig)
		if err != nil {
			return true, err
		}
		fmt.Println(value)
		return true, nil
	case opts.listConfig:
		lines, err := config.List()
		if err != nil {
			return true, err
		}
		for _, line := range lines {
			fmt.Println(line)
		}
		if named, from, sourceErr := source(); sourceErr != nil {
			return true, sourceErr
		} else if named != "" {
			fmt.Printf("\neffective provider: %s (from %s)\n", named, from)
		}
		return true, nil
	}
	return false, nil
}

func answer(result *provider.Result, structured bool) ([]byte, error) {
	if structured && result.Structured != nil {
		return json.MarshalIndent(result.Structured, "", "  ")
	}
	text := strings.TrimRight(result.Text, "\n")
	return []byte(text), nil
}

func promptFromTemplate(opts options) (string, string, error) {
	if opts.template == "" {
		if len(opts.vars) > 0 {
			return "", "", errors.New("--var requires --template")
		}
		return opts.prompt, "", nil
	}
	if opts.prompt != "" {
		return "", "", errors.New("use either --template or a prompt, not both")
	}

	prompt, err := templates.LoadPrompt(opts.template)
	if err != nil {
		return "", "", err
	}
	values, err := templates.Values(opts.vars)
	if err != nil {
		return "", "", err
	}
	for {
		rendered, missing, err := templates.Resolve(prompt, values)
		if err != nil {
			return "", "", err
		}
		if len(missing) == 0 {
			return rendered, prompt.Schema, nil
		}
		if opts.quiet || !term.IsTerminal(int(os.Stderr.Fd())) {
			names := make([]string, 0, len(missing))
			for _, variable := range missing {
				names = append(names, variable.Name)
			}
			return "", "", fmt.Errorf("prompt template %q needs --var for: %s", prompt.Name, strings.Join(names, ", "))
		}
		for _, variable := range missing {
			question := "value for " + variable.Name
			if variable.Description != "" {
				question += ": " + variable.Description
			}
			value, err := ui.Answer(question)
			if err != nil {
				return "", "", err
			}
			values[variable.Name] = value
		}
	}
}

// once runs the agent to one result, drawing whichever view the terminal allows.
func once(req provider.Request, opts options, agent provider.Provider) (*provider.Result, error) {
	ctx, stop := context.WithTimeout(context.Background(), opts.timeout)
	defer stop()

	events, err := agent.Run(ctx, req)
	if err != nil {
		return nil, err
	}

	var result *provider.Result
	switch {
	case !opts.quiet && term.IsTerminal(int(os.Stderr.Fd())):
		if result, err = ui.Run(events, stop); err != nil {
			return nil, err
		}
	case opts.quiet:
		result = ui.Drain(events, nil)
	default:
		result = ui.Drain(events, os.Stderr)
	}

	if result == nil {
		return nil, errors.New("the run ended without an answer")
	}
	if result.Failed {
		return nil, fmt.Errorf("%s: %s", agent.Name(), result.Reason)
	}
	return result, nil
}

// converse runs the agent until the answer fits, carrying back either a reply to
// its question or the reason its answer was rejected.
func converse(req provider.Request, strict map[string]any, opts options, agent provider.Provider) (*provider.Result, error) {
	human := !opts.quiet && term.IsTerminal(int(os.Stderr.Fd()))

	for round := 1; ; round++ {
		result, err := once(req, opts, agent)
		if err != nil {
			return nil, err
		}
		if strict == nil {
			return result, nil
		}

		if question := schema.Question(result.Structured); question != "" {
			if !human || round == rounds {
				return nil, asked{question}
			}
			reply, err := ui.Answer(question)
			if err != nil {
				return nil, err
			}
			req.Prompt, err = templates.WithAnsweredQuestion(req.Prompt, question, reply)
			if err != nil {
				return nil, err
			}
			continue
		}

		wrong := schema.Check(strict, result.Structured)
		if wrong == nil {
			return result, nil
		}
		if round == rounds {
			return nil, fmt.Errorf("%s: %s", agent.Name(), wrong)
		}
		req.Prompt, err = templates.WithRejectedAnswer(req.Prompt, wrong.Error())
		if err != nil {
			return nil, err
		}
	}
}

const rounds = 3

// asked leaves by its own exit code, so a script can tell "I need to know
// something" from "I failed".
type asked struct{ question string }

func (a asked) Error() string { return "the agent needs to know: " + a.question }

func run(opts options) error {
	if opts.capture {
		return pane.Capture()
	}
	if handled, err := settings(opts); handled {
		return err
	}
	if which := opts.show(); which != "" {
		return printSaved(which)
	}

	_, asJSON, _, err := wrapper(called())
	if err != nil {
		return err
	}
	if asJSON {
		opts.json = true
	}
	if opts.spec != "" && opts.schemaTemplate != "" {
		return errors.New("use either --schema or --schema-template, not both")
	}

	prompt, defaultSchema, err := promptFromTemplate(opts)
	if err != nil {
		return err
	}
	if opts.schemaTemplate == "" && opts.spec == "" {
		opts.schemaTemplate = defaultSchema
	}

	var shape map[string]any
	switch {
	case opts.schemaTemplate != "":
		named, loadErr := templates.LoadSchema(opts.schemaTemplate)
		if loadErr != nil {
			return loadErr
		}
		shape = named.Schema
	case opts.spec != "":
		if shape, err = schema.Resolve(opts.spec); err != nil {
			return err
		}
	case opts.json:
		shape = schema.Any()
	}

	agent, err := chosen(opts)
	if err != nil {
		return err
	}

	var input []byte
	switch {
	case opts.last:
		if input, err = pane.Last(); err != nil {
			return err
		}
	case opts.replay && !piped():
		if input, err = store.Input(); err != nil {
			return fmt.Errorf("nothing to replay: %w", err)
		}
	case piped():
		if input, err = io.ReadAll(os.Stdin); err != nil {
			return err
		}
	}

	if prompt == "" && opts.replay {
		saved, err := store.Prompt()
		if err != nil {
			return fmt.Errorf("no prompt to replay: %w", err)
		}
		prompt = string(saved)
	}
	if prompt == "" {
		return errors.New("say what to ask, or pass --help")
	}

	if err := store.SaveRun(input, prompt); err != nil {
		return err
	}

	here, _ := os.Getwd()
	loose, mayAsk := schema.Relaxed(shape)
	req := provider.Request{
		Prompt: prompt,
		Input:  string(input),
		Model:  opts.model,
		Schema: loose,
		Dir:    here,
	}
	if mayAsk {
		req.Prompt, err = templates.WithClarificationRule(req.Prompt, schema.Rule)
		if err != nil {
			return err
		}
	}

	result, err := converse(req, shape, opts, agent)
	if err != nil {
		return err
	}

	out, err := answer(result, shape != nil)
	if err != nil {
		return err
	}
	if err := store.SaveOutput(out); err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

func main() {
	var opts options
	err := command(&opts).Execute()
	var question asked
	switch {
	case err == nil:
		return
	case errors.Is(err, ui.ErrStopped):
		os.Exit(130)
	case errors.As(err, &question):
		fmt.Fprintln(os.Stderr, called()+": "+question.Error())
		os.Exit(3)
	default:
		fmt.Fprintln(os.Stderr, called()+": "+err.Error())
		os.Exit(1)
	}
}
