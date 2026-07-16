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
	commandAdd
	commandAddHelp
	commandNew
	commandNewHelp
	commandSummary
	commandIsProject
)

type invocation struct {
	kind       commandKind
	project    string
	source     string
	color      string
	priority   string
	start      string
	due        string
	projectSet bool
	global     bool
}

const helpText = `tasks — gestor local de tareas para terminal

Uso:
  tasks
  tasks init nombre.tasks
  tasks ai-prompt
  tasks import nombre.tasks [resultado.json|-]
  tasks add [--project ruta.tasks] [resultado.json|-]
  tasks new [--priority nivel] [--start AAAA-MM-DD] [--due AAAA-MM-DD] [--global|--project ruta.tasks] "Título"
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
  add                Agregar un lote JSON. Formato: tasks add --help.
  new                Crear una tarea rápidamente. Destino contextual; vea tasks new --help.
  summary            Mostrar tareas relevantes y, dentro de un proyecto, su Gantt.
  is-project         Validar si el directorio pertenece al árbol de un proyecto.
  help               Mostrar esta ayuda global.

Opciones:
  -h, --help         Mostrar esta ayuda global.
	--project ...      Destino .tasks explícito para add o new.
  --global           Usar explícitamente el almacén global con new.
  --color=...        Color de summary: auto, always o never.
  --no-color         Equivalente a --color=never.
`

const newHelpText = `tasks new — crear una tarea rápidamente

Uso:
  tasks new [opciones] "Título"
  tasks new [opciones] -- "-Título"
  tasks new -h
  tasks new --help

Destino:
  Sin selector       Dentro de un proyecto, usar el .tasks detectado; fuera, Global.
  --global           Usar el almacén global incluso dentro de un proyecto.
  --project ruta     Usar el archivo .tasks existente indicado por ruta.
  --project=ruta     Forma equivalente de indicar el proyecto.
  --global y --project son mutuamente excluyentes.

Opciones de la tarea:
  --priority nivel   none, low, medium, high o urgent (por defecto: none).
  --start fecha      Fecha de inicio en formato AAAA-MM-DD.
  --due fecha        Fecha de vencimiento en formato AAAA-MM-DD.
  Cada opción también admite la forma --opción=valor.
  --                 Finalizar las opciones; permite títulos que empiezan por guion.

El título es un único argumento, no puede quedar vacío y puede contener espacios.
La tarea se valida completamente antes de abrir o crear el destino. La salida es
JSON con el tipo de origen, su ruta cuando es un proyecto y el ID local creado.
`

const addHelpText = `tasks add — agregar tareas desde JSON

Uso:
  tasks add [resultado.json|-]
  tasks add --project ruta.tasks [resultado.json|-]
  tasks add -h
  tasks add --help

Destino:
  Sin --project      Agregar siempre al almacén global, aun dentro de un proyecto.
  --project ruta     Agregar al archivo .tasks existente indicado por ruta.
  --project=ruta     Forma equivalente de indicar el proyecto.

Entrada:
  Si se omite resultado.json o se usa -, leer un único objeto JSON desde stdin.
  Se rechazan campos desconocidos, contenido adicional y lotes sin tareas.

Formato tasks-project versión 1:
{
  "format": "tasks-project",
  "version": 1,
  "statuses": [
    {"key": "pending", "name": "Pendiente", "initial": true},
    {"key": "doing", "name": "En progreso"}
  ],
  "tasks": [
    {
      "key": "scope",
      "title": "Definir alcance",
      "status": "done",
      "priority": "high",
      "markdown": "Decisiones del proyecto."
    },
    {
      "key": "implementation",
      "title": "Implementar",
      "status": "pending",
      "start": "2026-07-20",
      "due": "2026-07-25",
      "subtasks": [{"title": "Implementar parser"}],
      "depends_on": ["scope"]
    }
  ]
}

Reglas del formato:
  statuses       Mapea claves a estados normales que ya existen por nombre exacto.
                 Debe declarar exactamente un initial y coincidir con el inicial real.
                 done y cancelled son claves reservadas para estados especiales.
  key            Es obligatoria, única dentro del lote y no se persiste.
  title          Es obligatorio y no puede quedar vacío.
  status         Clave declarada en statuses, done o cancelled; omitir usa initial.
  priority       none, low, medium, high o urgent; omitir usa none.
  markdown       Texto Markdown opcional.
  start, due     Fechas YYYY-MM-DD; due no puede preceder start.
  recurrence     daily, weekly:mon,thu, monthly:15, month-end o
                 monthly-weekday:first:mon / monthly-weekday:last:fri.
                 No puede combinarse con start ni due.
  subtasks       Lista opcional de objetos con title y status.
  depends_on     Lista opcional de key del mismo lote; no admite ciclos.

El lote es atómico: cualquier error impide todas sus altas. Repetir una entrada
correcta crea otro conjunto de tareas. La salida es JSON con el destino, los
conteos creados y la correspondencia entre cada key y su ID local.
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
	case "add":
		return parseAddInvocation(args[1:])
	case "new":
		return parseNewInvocation(args[1:])
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

func parseNewInvocation(arguments []string) (invocation, error) {
	if len(arguments) == 1 && (arguments[0] == "-h" || arguments[0] == "--help") {
		return invocation{kind: commandNewHelp}, nil
	}
	parsed := invocation{kind: commandNew, priority: "none"}
	var titleSet, prioritySet, startSet, dueSet bool
	endOfOptions := false
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		optionValue := func(alreadySet bool) (string, error) {
			if alreadySet || index+1 >= len(arguments) || arguments[index+1] == "" || strings.HasPrefix(arguments[index+1], "-") {
				return "", usageError(`tasks new [opciones] "Título"`)
			}
			index++
			return arguments[index], nil
		}
		switch {
		case endOfOptions:
			if titleSet {
				return invocation{}, usageError(`tasks new [opciones] "Título"`)
			}
			parsed.source, titleSet = argument, true
		case argument == "--":
			if titleSet || index+1 >= len(arguments) {
				return invocation{}, usageError(`tasks new [opciones] "Título"`)
			}
			endOfOptions = true
		case argument == "--global":
			if parsed.global || parsed.projectSet {
				return invocation{}, usageError(`tasks new [opciones] "Título"`)
			}
			parsed.global = true
		case argument == "--project":
			if parsed.global {
				return invocation{}, usageError(`tasks new [opciones] "Título"`)
			}
			value, err := optionValue(parsed.projectSet)
			if err != nil {
				return invocation{}, err
			}
			parsed.project, parsed.projectSet = value, true
		case strings.HasPrefix(argument, "--project="):
			value := strings.TrimPrefix(argument, "--project=")
			if parsed.global || parsed.projectSet || value == "" {
				return invocation{}, usageError(`tasks new [opciones] "Título"`)
			}
			parsed.project, parsed.projectSet = value, true
		case argument == "--priority":
			value, err := optionValue(prioritySet)
			if err != nil {
				return invocation{}, err
			}
			parsed.priority, prioritySet = value, true
		case strings.HasPrefix(argument, "--priority="):
			value := strings.TrimPrefix(argument, "--priority=")
			if prioritySet || value == "" {
				return invocation{}, usageError(`tasks new [opciones] "Título"`)
			}
			parsed.priority, prioritySet = value, true
		case argument == "--start":
			value, err := optionValue(startSet)
			if err != nil {
				return invocation{}, err
			}
			parsed.start, startSet = value, true
		case strings.HasPrefix(argument, "--start="):
			value := strings.TrimPrefix(argument, "--start=")
			if startSet || value == "" {
				return invocation{}, usageError(`tasks new [opciones] "Título"`)
			}
			parsed.start, startSet = value, true
		case argument == "--due":
			value, err := optionValue(dueSet)
			if err != nil {
				return invocation{}, err
			}
			parsed.due, dueSet = value, true
		case strings.HasPrefix(argument, "--due="):
			value := strings.TrimPrefix(argument, "--due=")
			if dueSet || value == "" {
				return invocation{}, usageError(`tasks new [opciones] "Título"`)
			}
			parsed.due, dueSet = value, true
		case strings.HasPrefix(argument, "-"):
			return invocation{}, unknownOption(argument)
		default:
			if titleSet {
				return invocation{}, usageError(`tasks new [opciones] "Título"`)
			}
			parsed.source, titleSet = argument, true
		}
	}
	if !titleSet {
		return invocation{}, usageError(`tasks new [opciones] "Título"`)
	}
	return parsed, nil
}

func parseAddInvocation(arguments []string) (invocation, error) {
	if len(arguments) == 1 && (arguments[0] == "-h" || arguments[0] == "--help") {
		return invocation{kind: commandAddHelp}, nil
	}
	parsed := invocation{kind: commandAdd}
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		switch {
		case argument == "--project":
			if parsed.projectSet || index+1 >= len(arguments) || arguments[index+1] == "" || strings.HasPrefix(arguments[index+1], "-") {
				return invocation{}, usageError("tasks add [--project ruta.tasks] [resultado.json|-]")
			}
			parsed.projectSet = true
			parsed.project = arguments[index+1]
			index++
		case strings.HasPrefix(argument, "--project="):
			if parsed.projectSet || strings.TrimPrefix(argument, "--project=") == "" {
				return invocation{}, usageError("tasks add [--project ruta.tasks] [resultado.json|-]")
			}
			parsed.projectSet = true
			parsed.project = strings.TrimPrefix(argument, "--project=")
		case strings.HasPrefix(argument, "-") && argument != "-":
			return invocation{}, unknownOption(argument)
		default:
			if parsed.source != "" {
				return invocation{}, usageError("tasks add [--project ruta.tasks] [resultado.json|-]")
			}
			parsed.source = argument
		}
	}
	return parsed, nil
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
