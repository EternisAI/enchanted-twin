// Component Generator - Automatically discovers and generates component registration code
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ComponentInfo represents a discovered component.
type ComponentInfo struct {
	ID         string
	Type       string
	MethodName string
	SourceFile string
	LineNumber int
}

// ComponentType mappings from method names to component types.
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

// discoverComponents walks the project directory and finds all loggerFactory calls.
func discoverComponents(projectRoot string) ([]ComponentInfo, error) {
	var components []ComponentInfo
	componentSet := make(map[string]ComponentInfo) // Use map to avoid duplicates

	err := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-Go files and vendor directories
		if !strings.HasSuffix(path, ".go") ||
			strings.Contains(path, "/vendor/") ||
			strings.Contains(path, "/.git/") ||
			strings.Contains(path, "/node_modules/") {
			return nil
		}

		// Parse the Go file
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			// Skip files that can't be parsed (might be invalid Go code)
			return nil
		}

		// Walk the AST to find loggerFactory calls
		ast.Inspect(node, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					// Check if it's a loggerFactory method call
					if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "loggerFactory" {
						methodName := sel.Sel.Name
						if strings.HasPrefix(methodName, "For") && len(call.Args) > 0 {
							// Extract the component ID from the first argument
							if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
								componentID := strings.Trim(lit.Value, `"`)

								// Get line number
								position := fset.Position(call.Pos())

								// Create component info
								component := ComponentInfo{
									ID:         componentID,
									Type:       getComponentType(methodName),
									MethodName: methodName,
									SourceFile: strings.TrimPrefix(path, projectRoot+"/"),
									LineNumber: position.Line,
								}

								// Use component ID as key to avoid duplicates
								componentSet[componentID] = component
							}
						}
					}
				}
			}
			return true
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Convert map to slice and sort by component ID
	for _, component := range componentSet {
		components = append(components, component)
	}

	sort.Slice(components, func(i, j int) bool {
		return components[i].ID < components[j].ID
	})

	return components, nil
}

// getComponentType returns the ComponentType constant for a given method name.
func getComponentType(methodName string) string {
	if componentType, exists := methodToType[methodName]; exists {
		return componentType
	}
	return "ComponentTypeUtility" // Default fallback
}

// generateComponentsFile creates the generated Go file with all discovered components.
func generateComponentsFile(outputFile string, components []ComponentInfo) error {
	// Ensure the directory exists
	err := os.MkdirAll(filepath.Dir(outputFile), 0o755)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate file content
	var builder strings.Builder

	// File header
	builder.WriteString(fmt.Sprintf("// Code generated by component-generator at %s. DO NOT EDIT.\n\n", time.Now().Format(time.RFC3339)))
	builder.WriteString("package bootstrap\n\n")

	// Generate RegisterAllKnownComponents function
	builder.WriteString("// RegisterAllKnownComponents registers all components discovered in the codebase\n")
	builder.WriteString("func RegisterAllKnownComponents(loggerFactory *LoggerFactory) {\n")

	// Group components by type for better organization
	componentsByType := make(map[string][]ComponentInfo)
	for _, component := range components {
		componentsByType[component.Type] = append(componentsByType[component.Type], component)
	}

	// Sort types for consistent output
	var types []string
	for componentType := range componentsByType {
		types = append(types, componentType)
	}
	sort.Strings(types)

	// Generate registration calls grouped by type
	for _, componentType := range types {
		typeComponents := componentsByType[componentType]

		// Add comment for component type
		typeName := strings.TrimPrefix(componentType, "ComponentType")
		builder.WriteString(fmt.Sprintf("\n\t// %s components\n", typeName))

		// Sort components within type
		sort.Slice(typeComponents, func(i, j int) bool {
			return typeComponents[i].ID < typeComponents[j].ID
		})

		for _, component := range typeComponents {
			builder.WriteString(fmt.Sprintf("\tloggerFactory.%s(\"%s\") // %s:%d\n",
				component.MethodName, component.ID, component.SourceFile, component.LineNumber))
		}
	}

	builder.WriteString("}\n\n")

	// Generate ComponentCatalog map
	builder.WriteString("// ComponentCatalog provides metadata about all known components\n")
	builder.WriteString("var ComponentCatalog = map[string]ComponentType{\n")

	for _, component := range components {
		builder.WriteString(fmt.Sprintf("\t\"%s\": %s,\n", component.ID, component.Type))
	}

	builder.WriteString("}\n\n")

	// Generate ComponentSources map for debugging
	builder.WriteString("// ComponentSources maps component IDs to their source locations\n")
	builder.WriteString("var ComponentSources = map[string]string{\n")

	for _, component := range components {
		builder.WriteString(fmt.Sprintf("\t\"%s\": \"%s:%d\",\n",
			component.ID, component.SourceFile, component.LineNumber))
	}

	builder.WriteString("}\n")

	// Write to file
	err = os.WriteFile(outputFile, []byte(builder.String()), 0o644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
