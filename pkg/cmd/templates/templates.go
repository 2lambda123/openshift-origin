package templates

import "strings"

func MainHelpTemplate() string {
	return decorate(mainHelpTemplate, false)
}

func MainUsageTemplate() string {
	return decorate(mainUsageTemplate, false)
}

func CliHelpTemplate() string {
	return decorate(cliHelpTemplate, false)
}

func CliUsageTemplate() string {
	return decorate(cliUsageTemplate, true)
}

func AdminHelpTemplate() string {
	return decorate(adminHelpTemplate, false)
}

func AdminUsageTemplate() string {
	return decorate(adminUsageTemplate, true)
}

func OptionsHelpTemplate() string {
	return decorate(optionsHelpTemplate, false)
}

func OptionsUsageTemplate() string {
	return decorate(optionsUsageTemplate, false)
}

func decorate(template string, trim bool) string {
	if trim && len(strings.Trim(template, " ")) > 0 {
		trimmed := strings.Trim(template, "\n")
		return funcs + trimmed
	}
	return funcs + template
}

const (
	// TODO: $isRootCmd should be done in code, not in the template
	funcs = `{{$isRootCmd := or (and (eq .Name "cli") (eq .Root.Name "openshift")) (eq .Name "osc") (and (eq .Name "admin") (eq .Root.Name "openshift")) (eq .Name "osadm")}}{{define "rootCli"}}{{if eq .Root.Name "osadm"}}osadm{{else}}{{if eq .Root.Name "osc"}}osc{{else}}openshift {{.Name}}{{end}}{{end}}{{end}}`

	mainHelpTemplate = `{{.Long | trim}}
{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

	mainUsageTemplate = `{{ $cmd := . }}
Usage: {{if .Runnable}}
  {{.UseLine}}{{if .HasFlags}} [options]{{end}}{{end}}{{if .HasSubCommands}}
  {{ .CommandPath}} <command>{{end}}{{if gt .Aliases 0}}

Aliases:
  {{.NameAndAliases}}{{end}}
{{ if .HasSubCommands}}
Available Commands: {{range .Commands}}{{if .Runnable}}
  {{rpad .Use .UsagePadding }} {{.Short}}{{end}}{{end}}
{{end}}
{{ if .HasLocalFlags}}Options:
{{.LocalFlags.FlagUsages}}{{end}}
{{ if .HasAnyPersistentFlags}}Global Options:
{{.AllPersistentFlags.FlagUsages}}{{end}}{{ if .HasSubCommands }}
Use "{{.Root.Name}} <command> --help" for more information about a given command.
{{end}}`

	cliHelpTemplate = `{{.Long | trim}}
{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

	cliUsageTemplate = `{{ $cmd := . }}{{$exposedFlags := exposed .}}{{ if .HasSubCommands}}
Available Commands: {{range .Commands}}{{if .Runnable}}{{if ne .Name "options"}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}
{{end}}
{{ if or .HasLocalFlags $exposedFlags.HasFlags}}Options:
{{ if .HasLocalFlags}}{{.LocalFlags.FlagUsages}}{{end}}{{ if $exposedFlags.HasFlags}}{{$exposedFlags.FlagUsages}}{{end}}
{{end}}{{ if not $isRootCmd}}Use "{{template "rootCli" .}} --help" for a list of all commands available in {{template "rootCli" .}}.
{{end}}{{ if .HasSubCommands }}Use "{{template "rootCli" .}} <command> --help" for more information about a given command.
{{end}}{{ if .HasAnyPersistentFlags}}Use "{{template "rootCli" .}} options" for a list of global command-line options (applies to all commands).
{{end}}`

	adminHelpTemplate = `{{.Long | trim}}
{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

	adminUsageTemplate = `{{ $cmd := . }}{{$exposedFlags := exposed .}}{{ if .HasSubCommands}}
Available Commands: {{range .Commands}}{{if .Runnable}}{{if ne .Name "options"}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}
{{end}}
{{ if or .HasLocalFlags $exposedFlags.HasFlags}}Options:
{{ if .HasLocalFlags}}{{.LocalFlags.FlagUsages}}{{end}}{{ if $exposedFlags.HasFlags}}{{$exposedFlags.FlagUsages}}{{end}}
{{end}}{{ if not $isRootCmd}}Use "{{template "rootCli" .}} --help" for a list of all commands available in {{template "rootCli" .}}.
{{end}}{{ if .HasSubCommands }}Use "{{template "rootCli" .}} <command> --help" for more information about a given command.
{{end}}{{ if .HasAnyPersistentFlags}}Use "{{template "rootCli" .}} options" for a list of global command-line options (applies to all commands).
{{end}}`

	optionsHelpTemplate = `{{ if .HasAnyPersistentFlags}}The following options can be passed to any command:

{{.AllPersistentFlags.FlagUsages}}{{end}}
`

	optionsUsageTemplate = ``
)
