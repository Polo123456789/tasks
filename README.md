# tasks

Gestor local de tareas en terminal. Cada proyecto es un único archivo SQLite portable `.tasks`.

## Uso

```sh
go install github.com/Polo123456789/tasks/cmd/tasks@latest
mkdir mi-proyecto && cd mi-proyecto
tasks init mi-proyecto.tasks
# En cualquier subdirectorio, `tasks` descubre el proyecto.
# Fuera de un proyecto abre las vistas globales.
```

Teclas: flechas o `hjkl`, `n` crea una tarea, `d` la envía a la papelera, `r` recarga y `q` sale.

Preview determinista:

```sh
go run ./cmd/ui-preview --screen kanban --fixture crowded
```

`--screen` admite `kanban`, `table`, `calendar`, `gantt` y `trash`; fixtures: `default`, `empty`, `crowded`, `error`.

## Desarrollo

```sh
go test ./...
go test -race ./...
go vet ./...
```

La arquitectura y decisiones están en [docs/adr](docs/adr). El núcleo no importa paquetes de terminal ni SQLite.
