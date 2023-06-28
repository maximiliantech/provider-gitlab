package main

import (
	_ "embed"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/go-cmp/cmp"
	"github.com/muvaf/typewriter/pkg/wrapper"
	"github.com/pkg/errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	genStatement = "// Code initially generated by generator."
)

//go:embed parameters_types.go.tmpl
var template string

func main() {
	var uri string
	var pathDir string
	var kind string
	var version string
	var postPath string
	var getPath string
	var patchPath string
	var putPath string
	var deletePath string

	// Defining arguments
	if len(os.Args) != 4 {
		//panic("Can not create execute. Please provide four parameters.")
		// GitLab ProjectAccessToken
		uri = "https://gitlab.com/gitlab-org/gitlab/-/raw/master/doc/api/openapi/openapi.yaml"
		pathDir = "apis/projects/v1alpha1"
		kind = "ProjectAccessToken"
		version = "v1alpha1"
		postPath = "/v4/projects/{id}/access_tokens"
		getPath = "/gists/{gist_id}"
		putPath = ""
		patchPath = ""
		deletePath = "/v4/projects/{id}/access_tokens/{token_id}"
	} else {
		uri = os.Args[1]
		pathDir = os.Args[2]
		kind = os.Args[3]
		version = os.Args[4]
		postPath = os.Args[5]
		patchPath = os.Args[6]
	}
	fmt.Println(postPath, getPath, patchPath, putPath, deletePath)

	// Create URL
	urlToOAPI, urlError := url.Parse(uri)
	if urlError != nil {
		panic(urlError)
	}

	// Create OpenAPI V3 spec from URL
	doc, err := openapi3.NewLoader().LoadFromURI(urlToOAPI)
	if err != nil {
		panic(err)
	}

	// Variables for injecting into template
	vars := make(map[string]*Field)

	// ------------------------------------------------------------------------------------------------
	postPathPar, postBodyPar := getPOST(doc, postPath)
	// ------------------------------------------------------------------------------------------------
	var patchPathPar map[string]*Field
	var patchBodyPar map[string]*Field
	if patchPath == "" && putPath != "" {
		patchPathPar, patchBodyPar = getPUT(doc, putPath)
	} else {
		patchPathPar, patchBodyPar = getPATCH(doc, patchPath)
	}
	// ------------------------------------------------------------------------------------------------
	var deletePathPar map[string]*Field
	if deletePath != "" {
		deletePathPar, _ = getDELETE(doc, deletePath)
	}

	// ------------------------------------------------------------------------------------------------

	// Append POST to vars
	for n, p := range postPathPar {
		vars[n] = p
	}
	for n, p := range postBodyPar {
		vars[n] = p
	}
	// ------------------------------------------------------------------------------------------------
	// Append PATCH to vars
	for n, p := range patchPathPar {
		// only add if it isn't added already
		if vars[n] == nil {
			vars[n] = p
		}
	}
	for n, p := range patchBodyPar {
		// only add if it isn't added already
		if vars[n] == nil {
			vars[n] = p
		}
		// Overwrite if variable from PATCH Body is required but already added one isn't
		if !vars[n].IsRequired && p.IsRequired {
			vars[n] = p
		}
	}
	// ------------------------------------------------------------------------------------------------
	// Append DELETE to vars
	if deletePathPar != nil {
		for n, p := range deletePathPar {
			// only add if it isn't added already
			if vars[n] == nil {
				vars[n] = p
			}
		}
	}
	// ------------------------------------------------------------------------------------------------
	// Check immutability
	for v, _ := range postBodyPar {
		if patchBodyPar[v] == nil {
			vars[v].IsImmutable = true
		}
	}

	parameters := map[string]any{
		"FIELDS":  vars,
		"VERSION": version,
		"KIND":    kind,
	}

	rootDir := "./"
	absRootDir, err := filepath.Abs(rootDir)

	if err != nil {
		panic(fmt.Sprintf("cannot calculate the absolute path with %s", rootDir))
	}

	pg := newParametersGenerator(absRootDir, pathDir)
	e := pg.generate(kind, parameters)
	if e != nil {
		panic(e)
	}
}

func typeCasting(typ *openapi3.Schema, required bool) string {
	switch typ.Type {
	case "integer":
		if required {
			return "int"
		} else {
			return "*int"
		}
	case "string":
		if required {
			return "string"
		} else {
			return "*string"
		}
	case "boolean":
		if required {
			return "bool"
		} else {
			return "*bool"
		}
	case "array":
		switch typ.Items.Value.Type {
		case "string":
			if required {
				return "[]string"
			} else {
				return "*[]string"
			}
		case "int":
			if required {
				return "[]int"
			} else {
				return "*[]int"
			}
		case "boolean":
			if required {
				return "[]bool"
			} else {
				return "*[]bool"
			}
		}
	}
	return ""
}

func isRequired(name string, requiredList []string) bool {
	for _, r := range requiredList {
		if r == name {
			return true
		}
	}
	return false
}

type parmetersGenerator struct {
	PublishingPath    string
	LicenseHeaderPath string
}

// This function will use a template, generate a file with the filename of kind
// and populate the values of the function parameter vars into it.
func (pg *parmetersGenerator) generate(kind string, vars map[string]any) error {
	// Read template file
	file := wrapper.NewFile("", "", template,
		wrapper.WithGenStatement(genStatement),
		wrapper.WithHeaderPath(pg.LicenseHeaderPath),
	)

	// Create filename based on the kind of the resource
	filename := fmt.Sprintf("%sparameters_types.go", strings.ToLower(kind))

	// Create path under which the instantiated file will be stored
	filePath := filepath.Join(pg.PublishingPath, filename)

	// Instantiate template file with variables and store it under filepath
	e := file.Write(filePath, vars, os.ModePerm)

	// return error or nil
	return errors.Wrap(e, fmt.Sprintf("cannot write file: %s", filename))
}

func newParametersGenerator(rootDir string, path string) *parmetersGenerator {
	return &parmetersGenerator{
		PublishingPath:    filepath.Join(rootDir, path),
		LicenseHeaderPath: filepath.Join(rootDir, "hack", "boilerplate.go.txt"),
	}
}

type Field struct {
	Name string

	GolangName string

	JSONName string

	Description string

	Type string

	Enum []interface{}

	EnumDefault interface{}

	IsRequired bool

	IsImmutable bool
}

func checkIfFieldsAreEqual(p1 *Field, p2 *Field) bool {
	return cmp.Equal(p1, p2)
}

func getPOST(doc *openapi3.T, path string) (map[string]*Field, map[string]*Field) {
	post := doc.Paths.Find(path).Post
	// Adding the POST path attributes to vars
	pathPar := make(map[string]*Field)
	for _, prop := range post.Parameters {
		goType := typeCasting(prop.Value.Schema.Value, prop.Value.Required)
		p := Field{
			Name:        prop.Value.Name,
			GolangName:  createGolangName(prop.Value.Name),
			JSONName:    createJSONStructTagName(prop.Value.Name),
			Description: prop.Value.Description,
			Type:        goType,
			Enum:        nil,
			EnumDefault: nil,
			IsRequired:  prop.Value.Required,
			IsImmutable: true,
		}
		pathPar[prop.Value.Name] = &p
	}

	bodyPar := make(map[string]*Field)
	// Adding the POST body attributes to vars
	for s, mediaType := range post.RequestBody.Value.Content {
		if s == "application/json" {
			// list of required fields in body
			requiredList := mediaType.Schema.Value.Required

			// Add
			for n, propContent := range mediaType.Schema.Value.Properties {
				isRequired := isRequired(n, requiredList)
				oapiType := propContent.Value
				goType := typeCasting(oapiType, isRequired)

				p := Field{
					Name:        n,
					GolangName:  createGolangName(n),
					JSONName:    createJSONStructTagName(n),
					Description: propContent.Value.Description,
					Type:        goType,
					Enum:        oapiType.Enum,
					EnumDefault: oapiType.Default,
					IsRequired:  isRequired,
					IsImmutable: false,
				}
				bodyPar[n] = &p
			}
		}
	}
	return pathPar, bodyPar
}

func getPATCH(doc *openapi3.T, path string) (map[string]*Field, map[string]*Field) {
	patch := doc.Paths.Find(path).Patch
	// Adding the path attributes to vars
	pathPar := make(map[string]*Field)
	for _, par := range patch.Parameters {
		goType := typeCasting(par.Value.Schema.Value, par.Value.Required)
		p := Field{
			Name:        par.Value.Name,
			GolangName:  createGolangName(par.Value.Name),
			JSONName:    createJSONStructTagName(par.Value.Name),
			Description: par.Value.Description,
			Type:        goType,
			Enum:        nil,
			EnumDefault: nil,
			IsRequired:  par.Value.Required,
			IsImmutable: true,
		}
		pathPar[par.Value.Name] = &p
	}

	bodyPar := make(map[string]*Field)
	// Adding the body attributes to vars
	for s, mediaType := range patch.RequestBody.Value.Content {
		if s == "application/json" {
			// list of required fields in body
			requiredList := mediaType.Schema.Value.Required

			// Add
			for n, propContent := range mediaType.Schema.Value.Properties {
				isRequired := isRequired(n, requiredList)
				oapiType := propContent.Value
				goType := typeCasting(oapiType, isRequired)

				p := Field{
					Name:        n,
					GolangName:  createGolangName(n),
					JSONName:    createJSONStructTagName(n),
					Description: propContent.Value.Description,
					Type:        goType,
					Enum:        oapiType.Enum,
					EnumDefault: oapiType.Default,
					IsRequired:  isRequired,
					IsImmutable: false,
				}
				bodyPar[n] = &p
			}
		}
	}
	return pathPar, bodyPar
}

func getPUT(doc *openapi3.T, path string) (map[string]*Field, map[string]*Field) {
	put := doc.Paths.Find(path).Put
	// Adding the path attributes to vars
	pathPar := make(map[string]*Field)
	for _, par := range put.Parameters {
		goType := typeCasting(par.Value.Schema.Value, par.Value.Required)
		p := Field{
			Name:        par.Value.Name,
			GolangName:  createGolangName(par.Value.Name),
			JSONName:    createJSONStructTagName(par.Value.Name),
			Description: par.Value.Description,
			Type:        goType,
			Enum:        nil,
			EnumDefault: nil,
			IsRequired:  par.Value.Required,
			IsImmutable: true,
		}
		pathPar[par.Value.Name] = &p
	}

	bodyPar := make(map[string]*Field)
	// Adding the body attributes to vars
	for s, mediaType := range put.RequestBody.Value.Content {
		if s == "application/json" {
			// list of required fields in body
			requiredList := mediaType.Schema.Value.Required

			// Add
			for n, propContent := range mediaType.Schema.Value.Properties {
				isRequired := isRequired(n, requiredList)
				oapiType := propContent.Value
				goType := typeCasting(oapiType, isRequired)

				p := Field{
					Name:        n,
					GolangName:  createGolangName(n),
					JSONName:    createJSONStructTagName(n),
					Description: propContent.Value.Description,
					Type:        goType,
					Enum:        oapiType.Enum,
					EnumDefault: oapiType.Default,
					IsRequired:  isRequired,
					IsImmutable: false,
				}
				bodyPar[n] = &p
			}
		}
	}
	return pathPar, bodyPar
}

func getDELETE(doc *openapi3.T, path string) (map[string]*Field, map[string]*Field) {
	del := doc.Paths.Find(path).Delete
	// Adding the path attributes to vars
	pathPar := make(map[string]*Field)
	for _, par := range del.Parameters {
		goType := typeCasting(par.Value.Schema.Value, par.Value.Required)
		p := Field{
			Name:        par.Value.Name,
			GolangName:  createGolangName(par.Value.Name),
			JSONName:    createJSONStructTagName(par.Value.Name),
			Description: par.Value.Description,
			Type:        goType,
			Enum:        nil,
			EnumDefault: nil,
			IsRequired:  par.Value.Required,
			IsImmutable: true,
		}
		pathPar[par.Value.Name] = &p
	}

	bodyPar := make(map[string]*Field)
	// Adding the body attributes to vars
	if del.RequestBody != nil {
		for s, mediaType := range del.RequestBody.Value.Content {
			if s == "application/json" {
				// list of required fields in body
				requiredList := mediaType.Schema.Value.Required

				// Add
				for n, propContent := range mediaType.Schema.Value.Properties {
					isRequired := isRequired(n, requiredList)
					oapiType := propContent.Value
					goType := typeCasting(oapiType, isRequired)

					p := Field{
						Name:        n,
						GolangName:  createGolangName(n),
						JSONName:    createJSONStructTagName(n),
						Description: propContent.Value.Description,
						Type:        goType,
						Enum:        oapiType.Enum,
						EnumDefault: oapiType.Default,
						IsRequired:  isRequired,
						IsImmutable: false,
					}
					bodyPar[n] = &p
				}
			}
		}
	} else {
		bodyPar = nil
	}

	return pathPar, bodyPar
}
