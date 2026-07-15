# ADR 0001: capas y SQLite autocontenido

El dominio no depende de UI ni persistencia. Application coordina puertos; los adaptadores implementan SQLite, registro, reloj, filesystem y editor. Cada `.tasks` es SQLite con journal DELETE, claves foráneas y versión optimista. Las consultas globales abren proyectos por separado; nunca usan ATTACH.
