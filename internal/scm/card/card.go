// Пакет card: нормализованная карточка SCM-репозитория по platform/scm-repository-card.
package card

// Card — обобщённая запись репозитория после ответа list API (без enrichment).
type Card struct {
	UID         string
	Name        string
	Path        string
	Description string
	Labels      []string
	Properties  map[string]any
}

// EmptyProperties возвращает пустой объект properties.
func EmptyProperties() map[string]any {
	return map[string]any{}
}
