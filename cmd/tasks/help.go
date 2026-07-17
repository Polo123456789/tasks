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
	commandExport
	commandExportHelp
	commandBackup
	commandBackupHelp
	commandRestore
	commandRestoreHelp
	commandDoctor
	commandDoctorHelp
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
	format     string
	projectSet bool
	global     bool
	structured bool
	force      bool
}

const helpText = `tasks — gestor local de tareas para terminal

Uso:
  tasks
  tasks init nombre.tasks
  tasks ai-prompt
  tasks import nombre.tasks [resultado.json|-]
  tasks add [--project ruta.tasks] [resultado.json|-]
  tasks new [--priority nivel] [--start AAAA-MM-DD] [--due AAAA-MM-DD] [--global|--project ruta.tasks] "Título"
  tasks export [--format json|markdown|csv] [--global|--project ruta.tasks]
  tasks backup [--global|--project ruta.tasks] respaldo.tasks.bak
  tasks restore respaldo.tasks.bak [--global|--project ruta.tasks] [--force]
  tasks doctor [--global|--project ruta.tasks] [--json]
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
  export             Exportar el almacén seleccionado sin modificarlo.
  backup             Crear un respaldo SQLite consistente y exclusivo.
  restore            Restaurar un respaldo validado sin sobrescribir por defecto.
  doctor             Diagnosticar esquema, integridad, permisos y registro.
  summary            Mostrar tareas relevantes y, dentro de un proyecto, su Gantt.
  is-project         Validar si el directorio pertenece al árbol de un proyecto.
  help               Mostrar esta ayuda global.

Opciones:
  -h, --help         Mostrar esta ayuda global.
  --project ...      Seleccionar explícitamente un archivo .tasks.
  --global           Seleccionar explícitamente el almacén global.
  --color=...        Color de summary: auto, always o never.
  --no-color         Equivalente a --color=never.
`

const exportHelpText = `tasks export — exportar datos sin modificar el almacén

Uso:
  tasks export [--format json|markdown|csv] [--project ruta.tasks]
  tasks export [--format json|markdown|csv] --global

Destino:
  Sin selector       Usar el proyecto detectado. Fuera de un proyecto se exige --global.
  --project ruta     Leer el archivo .tasks existente indicado.
  --global           Leer el almacén global existente de forma explícita.

Formatos:
  json               Formato portable tasks-project versión 1 (predeterminado).
  markdown           Lista legible con estado, planificación y relaciones.
  csv                Una fila por tarea, apta para hojas de cálculo.

La operación abre SQLite en modo de solo lectura y escribe el resultado en stdout.
Se rechazan bases en WAL o con sidecars activos para no crear ni modificarlos.
`

const backupHelpText = `tasks backup — crear un respaldo consistente

Uso:
  tasks backup [--project ruta.tasks] respaldo.tasks.bak
  tasks backup --global respaldo.tasks.bak

Sin selector se usa el proyecto detectado; fuera de uno se exige --global.
El destino no puede existir; se recomienda la extensión .tasks.bak para que no
se descubra como un proyecto activo. El respaldo se publica
atómicamente desde una instantánea consistente de SQLite y usa permisos privados.
La fuente debe estar cerrada, en modo DELETE y sin sidecars activos.
`

const restoreHelpText = `tasks restore — restaurar un respaldo validado

Uso:
  tasks restore respaldo.tasks.bak [--project ruta.tasks] [--force]
  tasks restore respaldo.tasks.bak --global [--force]

Sin selector se restaura el proyecto detectado; fuera de uno debe indicarse
--project o --global. Un destino nuevo se crea de forma atómica. Si ya existe,
se rechaza salvo que --force autorice explícitamente el reemplazo. El respaldo
se valida y, si es compatible, se migra en un archivo temporal antes de publicar.
Fuente y destino deben estar cerrados, en modo DELETE y sin sidecars activos.
`

const doctorHelpText = `tasks doctor — diagnosticar almacenes locales

Uso:
  tasks doctor [--project ruta.tasks] [--json]
  tasks doctor --global [--json]

Sin selector se diagnostica el proyecto detectado; el almacén global siempre se
selecciona con --global. Revisa versión y tablas, integrity_check, claves foráneas,
permisos y, en global, el registro y sus proyectos no disponibles. --json produce
una salida estructurada. Doctor nunca crea, migra, repara ni poda datos.
Las bases en WAL o con sidecars activos se reportan como reparables sin abrirlas.
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
	case "export":
		return parseDataInvocation(commandExport, args[1:])
	case "backup":
		return parseDataInvocation(commandBackup, args[1:])
	case "restore":
		return parseDataInvocation(commandRestore, args[1:])
	case "doctor":
		return parseDataInvocation(commandDoctor, args[1:])
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

func parseDataInvocation(kind commandKind, arguments []string) (invocation, error) {
	helpKinds := map[commandKind]commandKind{commandExport: commandExportHelp, commandBackup: commandBackupHelp, commandRestore: commandRestoreHelp, commandDoctor: commandDoctorHelp}
	if len(arguments) == 1 && (arguments[0] == "-h" || arguments[0] == "--help") {
		return invocation{kind: helpKinds[kind]}, nil
	}
	parsed := invocation{kind: kind}
	formatSet := false
	if kind == commandExport {
		parsed.format = "json"
	}
	positional := ""
	endOptions := false
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		optionValue := func(alreadySet bool) (string, error) {
			if alreadySet || index+1 >= len(arguments) || arguments[index+1] == "" || strings.HasPrefix(arguments[index+1], "-") {
				return "", dataUsageError(kind)
			}
			index++
			return arguments[index], nil
		}
		switch {
		case endOptions:
			if positional != "" {
				return invocation{}, dataUsageError(kind)
			}
			positional = argument
		case argument == "--":
			if endOptions {
				return invocation{}, dataUsageError(kind)
			}
			endOptions = true
		case argument == "--global":
			if parsed.global || parsed.projectSet {
				return invocation{}, dataUsageError(kind)
			}
			parsed.global = true
		case argument == "--project":
			if parsed.global {
				return invocation{}, dataUsageError(kind)
			}
			value, err := optionValue(parsed.projectSet)
			if err != nil {
				return invocation{}, err
			}
			parsed.project, parsed.projectSet = value, true
		case strings.HasPrefix(argument, "--project="):
			value := strings.TrimPrefix(argument, "--project=")
			if parsed.global || parsed.projectSet || value == "" {
				return invocation{}, dataUsageError(kind)
			}
			parsed.project, parsed.projectSet = value, true
		case kind == commandExport && argument == "--format":
			value, err := optionValue(formatSet)
			if err != nil {
				return invocation{}, err
			}
			parsed.format, formatSet = value, true
		case kind == commandExport && strings.HasPrefix(argument, "--format="):
			value := strings.TrimPrefix(argument, "--format=")
			if value == "" || formatSet {
				return invocation{}, dataUsageError(kind)
			}
			parsed.format, formatSet = value, true
		case kind == commandDoctor && argument == "--json":
			if parsed.structured {
				return invocation{}, dataUsageError(kind)
			}
			parsed.structured = true
		case kind == commandRestore && argument == "--force":
			if parsed.force {
				return invocation{}, dataUsageError(kind)
			}
			parsed.force = true
		case strings.HasPrefix(argument, "-"):
			return invocation{}, unknownOption(argument)
		default:
			if positional != "" {
				return invocation{}, dataUsageError(kind)
			}
			positional = argument
		}
	}
	if kind == commandExport {
		if positional != "" || (parsed.format != "json" && parsed.format != "markdown" && parsed.format != "csv") {
			return invocation{}, dataUsageError(kind)
		}
	} else if kind == commandDoctor {
		if positional != "" {
			return invocation{}, dataUsageError(kind)
		}
	} else if positional == "" {
		return invocation{}, dataUsageError(kind)
	}
	parsed.source = positional
	return parsed, nil
}

func dataUsageError(kind commandKind) error {
	usage := map[commandKind]string{
		commandExport:  "tasks export [--format json|markdown|csv] [--global|--project ruta.tasks]",
		commandBackup:  "tasks backup [--global|--project ruta.tasks] respaldo.tasks.bak",
		commandRestore: "tasks restore respaldo.tasks.bak [--global|--project ruta.tasks] [--force]",
		commandDoctor:  "tasks doctor [--global|--project ruta.tasks] [--json]",
	}
	return usageError(usage[kind])
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
