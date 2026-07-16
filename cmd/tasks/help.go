package main

import (
	"fmt"
	"strings"
)

type commandKind int

const (
	commandTUI commandKind = iota
	commandHelp
	commandInit
	commandAIPrompt
	commandImport
	commandSummary
	commandIsProject
)

type invocation struct {
	kind    commandKind
	project string
	source  string
	color   string
}

const helpText = `tasks — gestor local de tareas para terminal

Uso:
  tasks
  tasks init nombre.tasks
  tasks ai-prompt
  tasks import nombre.tasks [resultado.json|-]
  tasks summary [--color=auto|always|never]
  tasks is-project
  tasks help
  tasks -h
  tasks --help

Comandos:
  (sin argumentos)  Abrir la TUI en modo local o global.
  init               Crear un proyecto nuevo y abrirlo.
  ai-prompt          Imprimir el prompt para convertir una conversación a JSON.
  import             Crear un proyecto nuevo desde JSON en un archivo o stdin.
  summary            Mostrar tareas relevantes y, dentro de un proyecto, su Gantt.
  is-project         Validar si el directorio pertenece al árbol de un proyecto.
  help               Mostrar esta ayuda global.

Opciones:
  -h, --help         Mostrar esta ayuda global.
  --color=...        Color de summary: auto, always o never.
  --no-color         Equivalente a --color=never.
`

func parseInvocation(args []string) (invocation, error) {
	if len(args) == 0 {
		return invocation{kind: commandTUI}, nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		if err := rejectUnknownOptions(args[1:], nil); err != nil {
			return invocation{}, err
		}
		if len(args) != 1 {
			return invocation{}, usageError("tasks help")
		}
		return invocation{kind: commandHelp}, nil
	case "ai-prompt":
		if err := rejectUnknownOptions(args[1:], nil); err != nil {
			return invocation{}, err
		}
		if len(args) != 1 {
			return invocation{}, usageError("tasks ai-prompt")
		}
		return invocation{kind: commandAIPrompt}, nil
	case "is-project":
		if err := rejectUnknownOptions(args[1:], nil); err != nil {
			return invocation{}, err
		}
		if len(args) != 1 {
			return invocation{}, usageError("tasks is-project")
		}
		return invocation{kind: commandIsProject}, nil
	case "init":
		if err := rejectUnknownOptions(args[1:], nil); err != nil {
			return invocation{}, err
		}
		if len(args) != 2 {
			return invocation{}, usageError("tasks init nombre.tasks")
		}
		return invocation{kind: commandInit, project: args[1]}, nil
	case "import":
		if err := rejectUnknownOptions(args[1:], func(index int, argument string) bool {
			return index == 1 && argument == "-"
		}); err != nil {
			return invocation{}, err
		}
		if len(args) < 2 || len(args) > 3 {
			return invocation{}, usageError("tasks import nombre.tasks [resultado.json|-]")
		}
		invocation := invocation{kind: commandImport, project: args[1]}
		if len(args) == 3 {
			invocation.source = args[2]
		}
		return invocation, nil
	case "summary":
		parsed := invocation{kind: commandSummary, color: "auto"}
		for _, argument := range args[1:] {
			switch argument {
			case "--color=auto":
				parsed.color = "auto"
			case "--color=always":
				parsed.color = "always"
			case "--color=never", "--no-color":
				parsed.color = "never"
			default:
				if strings.HasPrefix(argument, "-") {
					return invocation{}, unknownOption(argument)
				}
				return invocation{}, usageError("tasks summary [--color=auto|always|never]")
			}
		}
		return parsed, nil
	default:
		if strings.HasPrefix(args[0], "-") {
			return invocation{}, unknownOption(args[0])
		}
		return invocation{}, fmt.Errorf("comando desconocido %q. %s", args[0], helpSuggestion)
	}
}

const helpSuggestion = `Use "tasks help" para ver los comandos disponibles.`

func rejectUnknownOptions(arguments []string, allowed func(int, string) bool) error {
	for index, argument := range arguments {
		if strings.HasPrefix(argument, "-") && (allowed == nil || !allowed(index, argument)) {
			return unknownOption(argument)
		}
	}
	return nil
}

func unknownOption(option string) error {
	return fmt.Errorf("opción desconocida %q. %s", option, helpSuggestion)
}

func usageError(usage string) error {
	return fmt.Errorf("uso: %s\n%s", usage, helpSuggestion)
}
