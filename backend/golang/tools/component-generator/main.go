package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type ComponentInfo struct {
	ID         string
	Type       string
	MethodName string
	SourceFile string
	LineNumber int
}

var methodToType = map[string]string{
	"ForComponent":   "ComponentTypeUtility",
	"ForService":     "ComponentTypeService",
	"ForManager":     "ComponentTypeManager",
	"ForHandler":     "ComponentTypeHandler",
	"ForResolver":    "ComponentTypeResolver",
	"ForRepository":  "ComponentTypeRepository",
	"ForWorker":      "ComponentTypeWorker",
	"ForClient":      "ComponentTypeClient",
	"ForServer":      "ComponentTypeServer",
	"ForMiddleware":  "ComponentTypeMiddleware",
	"ForAI":          "ComponentTypeAI",
	"ForAnonymizer":  "ComponentTypeAnonymizer",
	"ForEmbedding":   "ComponentTypeEmbedding",
	"ForCompletions": "ComponentTypeCompletions",
	"ForProcessor":   "ComponentTypeProcessor",
	"ForWorkflow":    "ComponentTypeWorkflow",
	"ForIntegration": "ComponentTypeIntegration",
	"ForParser":      "ComponentTypeParser",
	"ForTelegram":    "ComponentTypeTelegram",
	"ForWhatsApp":    "ComponentTypeWhatsApp",
	"ForSlack":       "ComponentTypeSlack",
	"ForGmail":       "ComponentTypeGmail",
	"ForMCP":         "ComponentTypeMCP",
	"ForDatabase":    "ComponentTypeDatabase",
	"ForNATS":        "ComponentTypeNATS",
	"ForTemporal":    "ComponentTypeTemporal",
	"ForDirectory":   "ComponentTypeDirectory",
	"ForIdentity":    "ComponentTypeIdentity",
	"ForAuth":        "ComponentTypeAuth",
	"ForOAuth":       "ComponentTypeOAuth",
	"ForChat":        "ComponentTypeChat",
	"ForMemory":      "ComponentTypeMemory",
	"ForTwinChat":    "ComponentTypeTwinChat",
	"ForTTS":         "ComponentTypeTTS",
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <project-root>\n", os.Args[0])
		os.Exit(1)
	}

	projectRoot := os.Args[1]
	components, err := discoverComponents(projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error discovering components: %v\n", err)
		os.Exit(1)
	}

	outputFile := filepath.Join(projectRoot, "pkg/bootstrap/components_generated.go")
	err = generateComponentsFile(outputFile, components)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating components file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %d components in %s\n", len(components), outputFile)
}

func discoverComponents(projectRoot string) ([]ComponentInfo, error) {
	var components []ComponentInfo
	componentSet := make(map[string]ComponentInfo)
	packages := make(map[string][]*ast.File)
	fileMap := make(map[*ast.File]string)
	fset := token.NewFileSet()

	err := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".go") ||
			strings.Contains(path, filepath.FromSlash("vendor")) ||
			strings.Contains(path, filepath.FromSlash(".git")) ||
			strings.Contains(path, filepath.FromSlash("node_modules")) {
			return nil
		}

		if strings.HasSuffix(path, "_test.go") ||
			strings.Contains(filepath.Base(path), "generated") ||
			strings.Contains(filepath.Base(path), "gen_") ||
			strings.Contains(filepath.Base(path), "_gen") ||
			strings.Contains(path, "components_generated.go") {
			return nil
		}

		if info.Size() > 0 {
			file, err := os.Open(path)
			if err == nil {
				defer func() {
					if closeErr := file.Close(); closeErr != nil {
						// Log close error but don't fail the operation
						fmt.Fprintf(os.Stderr, "Warning: failed to close file %s: %v\n", path, closeErr)
					}
				}()
				buffer := make([]byte, 1024)
				n, _ := file.Read(buffer)
				if n > 0 && strings.Contains(string(buffer[:n]), "Code generated") {
					return nil
				}
			}
		}

		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil
		}

		pkgName := node.Name.Name
		if packages[pkgName] == nil {
			packages[pkgName] = make([]*ast.File, 0)
		}
		packages[pkgName] = append(packages[pkgName], node)
		fileMap[node] = path

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort package names for deterministic processing
	var pkgNames []string
	for pkgName := range packages {
		pkgNames = append(pkgNames, pkgName)
	}
	sort.Strings(pkgNames)

	for _, pkgName := range pkgNames {
		files := packages[pkgName]
		conf := types.Config{
			Importer: importer.Default(),
			Error:    func(err error) {},
		}

		info := &types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
		}

		// Check package types - errors are handled by the Error function in conf
		_, _ = conf.Check(pkgName, fset, files, info)

		// Sort files within the package for deterministic processing
		sort.Slice(files, func(i, j int) bool {
			return fileMap[files[i]] < fileMap[files[j]]
		})

		for _, file := range files {
			ast.Inspect(file, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						methodName := sel.Sel.Name
						if strings.HasPrefix(methodName, "For") && len(call.Args) > 0 {
							if isLoggerFactoryType(sel.X, info) {
								if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
									componentID, err := strconv.Unquote(lit.Value)
									if err != nil {
										return true
									}

									position := fset.Position(call.Pos())
									relPath := strings.TrimPrefix(fileMap[file], projectRoot+string(filepath.Separator))

									component := ComponentInfo{
										ID:         componentID,
										Type:       getComponentType(methodName),
										MethodName: methodName,
										SourceFile: relPath,
										LineNumber: position.Line,
									}

									componentSet[componentID] = component
								}
							}
						}
					}
				}
				return true
			})
		}
	}

	for _, component := range componentSet {
		components = append(components, component)
	}

	sort.Slice(components, func(i, j int) bool {
		return components[i].ID < components[j].ID
	})

	return components, nil
}

func isLoggerFactoryType(expr ast.Expr, info *types.Info) bool {
	if typeAndValue, exists := info.Types[expr]; exists {
		typeStr := typeAndValue.Type.String()
		if strings.Contains(typeStr, "LoggerFactory") ||
			strings.HasSuffix(typeStr, "bootstrap.LoggerFactory") ||
			strings.HasSuffix(typeStr, "*LoggerFactory") {
			return true
		}
	}

	switch x := expr.(type) {
	case *ast.Ident:
		return strings.Contains(x.Name, "LoggerFactory") ||
			x.Name == "loggerFactory" ||
			x.Name == "factory"
	case *ast.SelectorExpr:
		return x.Sel.Name == "LoggerFactory"
	}

	return false
}

func getComponentType(methodName string) string {
	if componentType, exists := methodToType[methodName]; exists {
		return componentType
	}
	return "ComponentTypeUtility"
}

func generateComponentsFile(outputFile string, components []ComponentInfo) error {
	err := os.MkdirAll(filepath.Dir(outputFile), 0o755)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	var builder strings.Builder

	builder.WriteString("// Code generated by component-generator. DO NOT EDIT.\n\n")
	builder.WriteString("package bootstrap\n\n")

	builder.WriteString("// RegisterAllKnownComponents registers all components discovered in the codebase\n")
	builder.WriteString("func RegisterAllKnownComponents(loggerFactory *LoggerFactory) {\n")

	componentsByType := make(map[string][]ComponentInfo)
	for _, component := range components {
		componentsByType[component.Type] = append(componentsByType[component.Type], component)
	}

	var types []string
	for componentType := range componentsByType {
		types = append(types, componentType)
	}
	sort.Strings(types)

	for _, componentType := range types {
		typeComponents := componentsByType[componentType]

		typeName := strings.TrimPrefix(componentType, "ComponentType")
		builder.WriteString(fmt.Sprintf("\n\t// %s components\n", typeName))

		sort.Slice(typeComponents, func(i, j int) bool {
			return typeComponents[i].ID < typeComponents[j].ID
		})

		for _, component := range typeComponents {
			builder.WriteString(fmt.Sprintf("\tloggerFactory.%s(\"%s\") // %s:%d\n",
				component.MethodName, component.ID, component.SourceFile, component.LineNumber))
		}
	}

	builder.WriteString("}\n\n")

	builder.WriteString("// ComponentCatalog provides metadata about all known components\n")
	builder.WriteString("var ComponentCatalog = map[string]ComponentType{\n")

	for _, component := range components {
		builder.WriteString(fmt.Sprintf("\t\"%s\": %s,\n", component.ID, component.Type))
	}

	builder.WriteString("}\n\n")

	builder.WriteString("// ComponentSources maps component IDs to their source locations\n")
	builder.WriteString("var ComponentSources = map[string]string{\n")

	for _, component := range components {
		builder.WriteString(fmt.Sprintf("\t\"%s\": \"%s:%d\",\n",
			component.ID, component.SourceFile, component.LineNumber))
	}

	builder.WriteString("}\n")

	formattedCode, err := format.Source([]byte(builder.String()))
	if err != nil {
		return fmt.Errorf("failed to format generated code: %w", err)
	}

	err = os.WriteFile(outputFile, formattedCode, 0o644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
