package keymap

import (
	"strings"
	"testing"
)

func TestFooterChangesWithScreenAndSelection(t *testing.T) {
	tests := []struct {
		name    string
		context Context
		want    []string
		reject  []string
	}{
		{
			name:    "empty task view",
			context: Context{View: 0},
			want:    []string{"n nueva tarea", "/ título", "F1 mapa general"},
			reject:  []string{"e título", "d papelera", "Subtarea"},
		},
		{
			name:    "gantt task",
			context: Context{View: 3, HasTask: true, HasSubtask: true, HasDependency: true},
			want:    []string{"PgUp/PgDn cambiar mes", ",/. desplazar 7 días", "G quitar dependencia", "J/K seleccionar"},
		},
		{
			name:    "trash selection",
			context: Context{View: 4, HasTask: true},
			want:    []string{"u restaurar", "q salir"},
			reject:  []string{"Filtros", "n nueva tarea", "e título"},
		},
		{
			name:    "normal status",
			context: Context{View: 5, NormalStatus: true},
			want:    []string{"a crear estado", "e renombrar", "i hacer inicial", "[/] reordenar", "d eliminar"},
			reject:  []string{"Filtros", "n nueva tarea"},
		},
		{
			name:    "special status",
			context: Context{View: 5},
			want:    []string{"a crear estado"},
			reject:  []string{"e renombrar", "d eliminar"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			footer := Footer(test.context)
			for _, expected := range test.want {
				if !strings.Contains(footer, expected) {
					t.Fatalf("missing %q:\n%s", expected, footer)
				}
			}
			for _, rejected := range test.reject {
				if strings.Contains(footer, rejected) {
					t.Fatalf("unexpected %q:\n%s", rejected, footer)
				}
			}
		})
	}
}
