//go:build cssstub

package interaction

import (
	"errors"
)

type TemplateEngine interface {
	HandleSafe(input TemplateInput) (string, error)
}

type TemplateInput struct {
	Contents interface{}
	Template Template
}

type Template struct {
	TemplateFile string
}

type HtmlViewEntry struct {
	Route    InteractionRoute
	FilePath string
}

type HtmlViewHandler struct {
	idpIndex       string
	templateEngine TemplateEngine
	templates      []HtmlViewEntry
}

func NewHtmlViewHandler(index InteractionRoute, templateEngine TemplateEngine, templates []HtmlViewEntry) *HtmlViewHandler {
	idpIndex, _ := index.GetPath(nil)
	return &HtmlViewHandler{
		idpIndex:       idpIndex,
		templateEngine: templateEngine,
		templates:      templates,
	}
}

func (h *HtmlViewHandler) CanHandle(input InteractionHandlerInput) error {
	// Placeholder for method and preference checking
	return nil
}

func (h *HtmlViewHandler) Handle(input InteractionHandlerInput) (*Representation, error) {
	template := h.findTemplate("") // Placeholder for target path
	contents := map[string]interface{}{
		"idpIndex":       h.idpIndex,
		"authenticating": input.OidcInteraction != nil,
	}
	result, _ := h.templateEngine.HandleSafe(TemplateInput{
		Contents: contents,
		Template: Template{TemplateFile: template},
	})
	return &Representation{
		Data: result,
	}, nil
}

func (h *HtmlViewHandler) findTemplate(target string) string {
	for _, template := range h.templates {
		// Placeholder for path matching
		return template.FilePath
	}
	panic(errors.New("template not found"))
}
