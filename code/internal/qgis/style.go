package qgis

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"waste-routing/internal/models"
)

const (
	styleKeyProperty   = "style_key"
	styleColorProperty = "style_color"
	styleLabelProperty = "style_label"
	unassignedStyleKey = "unassigned"
)

var qgisPalette = []string{
	"#1f77b4",
	"#ff7f0e",
	"#2ca02c",
	"#d62728",
	"#9467bd",
	"#8c564b",
	"#e377c2",
	"#17becf",
	"#bcbd22",
	"#7f7f7f",
	"#006d77",
	"#9d4edd",
}

type symbolStyle struct {
	Key   string
	Label string
	Color string
}

type styleCatalog struct {
	ordered []symbolStyle
	byKey   map[string]symbolStyle
}

type sourceAssignmentStyle struct {
	targetType string
	targetID   string
}

func buildStyleCatalog(transfers []PointFeature, plants []PointFeature) styleCatalog {
	catalog := styleCatalog{
		ordered: make([]symbolStyle, 0, len(transfers)+len(plants)+1),
		byKey:   make(map[string]symbolStyle, len(transfers)+len(plants)+1),
	}

	colorIndex := 0
	for _, point := range transfers {
		catalog.addFacility(models.ObjectTypeTransferStation, point, colorIndex)
		colorIndex++
	}
	for _, point := range plants {
		catalog.addFacility(models.ObjectTypePlant, point, colorIndex)
		colorIndex++
	}
	catalog.add(symbolStyle{
		Key:   unassignedStyleKey,
		Label: "unassigned",
		Color: "#9e9e9e",
	})

	return catalog
}

func (catalog *styleCatalog) addFacility(objectType models.ObjectType, point PointFeature, colorIndex int) {
	id := propertyString(point.Properties, "id")
	if id == "" {
		return
	}

	catalog.add(symbolStyle{
		Key:   facilityStyleKey(objectType, id),
		Label: facilityLabel(objectType, id, point.Properties),
		Color: qgisPalette[colorIndex%len(qgisPalette)],
	})
}

func (catalog *styleCatalog) add(style symbolStyle) {
	if _, exists := catalog.byKey[style.Key]; exists {
		return
	}
	catalog.ordered = append(catalog.ordered, style)
	catalog.byKey[style.Key] = style
}

func (catalog styleCatalog) styleForKey(key string) symbolStyle {
	if style, ok := catalog.byKey[key]; ok {
		return style
	}
	return catalog.byKey[unassignedStyleKey]
}

func (catalog styleCatalog) facilityStyles() []symbolStyle {
	styles := make([]symbolStyle, 0, len(catalog.ordered))
	for _, style := range catalog.ordered {
		if style.Key == unassignedStyleKey {
			continue
		}
		styles = append(styles, style)
	}
	return styles
}

func (catalog styleCatalog) stylesForType(objectType models.ObjectType) []symbolStyle {
	prefix := string(objectType) + ":"
	styles := make([]symbolStyle, 0)
	for _, style := range catalog.ordered {
		if strings.HasPrefix(style.Key, prefix) {
			styles = append(styles, style)
		}
	}
	return styles
}

func applyFacilityPointStyles(points []PointFeature, objectType models.ObjectType, catalog styleCatalog) {
	for index := range points {
		id := propertyString(points[index].Properties, "id")
		style := catalog.styleForKey(facilityStyleKey(objectType, id))
		applyStyleProperties(points[index].Properties, style)
	}
}

func applyWasteSourceAssignmentStyles(points []PointFeature, assignments map[string]sourceAssignmentStyle, catalog styleCatalog) {
	for index := range points {
		sourceID := propertyString(points[index].Properties, "id")
		style := catalog.styleForKey(unassignedStyleKey)

		if assignment, ok := assignments[sourceID]; ok {
			key := targetStyleKey(assignment.targetType, assignment.targetID)
			style = catalog.styleForKey(key)
			points[index].Properties["assignment_type"] = assignment.targetType
			points[index].Properties["destination_id"] = assignment.targetID
			points[index].Properties["destination_name"] = style.Label
		} else {
			points[index].Properties["assignment_type"] = ""
			points[index].Properties["destination_id"] = ""
			points[index].Properties["destination_name"] = ""
		}

		applyStyleProperties(points[index].Properties, style)
	}
}

func applySourceAssignmentLineStyles(lines []LineFeature, catalog styleCatalog) {
	for index := range lines {
		key := targetStyleKey(
			propertyString(lines[index].Properties, "target_type"),
			propertyString(lines[index].Properties, "target_id"),
		)
		applyStyleProperties(lines[index].Properties, catalog.styleForKey(key))
	}
}

func applyTransferFlowLineStyles(lines []LineFeature, catalog styleCatalog) {
	for index := range lines {
		key := facilityStyleKey(models.ObjectTypePlant, propertyString(lines[index].Properties, "plant_id"))
		applyStyleProperties(lines[index].Properties, catalog.styleForKey(key))
	}
}

func sourceAssignmentIndexFromLines(lines []LineFeature) map[string]sourceAssignmentStyle {
	assignments := make(map[string]sourceAssignmentStyle, len(lines))
	for _, line := range lines {
		sourceID := propertyString(line.Properties, "source_id")
		if sourceID == "" {
			continue
		}
		assignments[sourceID] = sourceAssignmentStyle{
			targetType: propertyString(line.Properties, "target_type"),
			targetID:   propertyString(line.Properties, "target_id"),
		}
	}
	return assignments
}

func applyStyleProperties(properties map[string]any, style symbolStyle) {
	properties[styleKeyProperty] = style.Key
	properties[styleColorProperty] = style.Color
	properties[styleLabelProperty] = style.Label
}

func facilityStyleKey(objectType models.ObjectType, id string) string {
	return string(objectType) + ":" + strings.TrimSpace(id)
}

func targetStyleKey(targetType string, targetID string) string {
	return normalizeTargetType(targetType) + ":" + strings.TrimSpace(targetID)
}

func facilityLabel(objectType models.ObjectType, id string, properties map[string]any) string {
	if name := propertyString(properties, "name"); name != "" {
		return name
	}
	return fmt.Sprintf("%s %s", objectType, id)
}

func propertyString(properties map[string]any, key string) string {
	value, ok := properties[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func writeQGISStyles(paths OutputPaths, catalog styleCatalog) error {
	if err := writeCategorizedMarkerStyle(paths.WasteSourcesStyle, "circle", 1.2, catalog.ordered); err != nil {
		return fmt.Errorf("write waste source style: %w", err)
	}
	if err := writeCategorizedMarkerStyle(paths.TransferStationsStyle, "triangle", 4.5, catalog.stylesForType(models.ObjectTypeTransferStation)); err != nil {
		return fmt.Errorf("write transfer station style: %w", err)
	}
	if err := writeCategorizedMarkerStyle(paths.PlantsStyle, "star", 7.0, catalog.stylesForType(models.ObjectTypePlant)); err != nil {
		return fmt.Errorf("write plant style: %w", err)
	}
	if err := writeCategorizedLineStyle(paths.SourceAssignmentsStyle, 0.45, catalog.facilityStyles()); err != nil {
		return fmt.Errorf("write source assignment style: %w", err)
	}
	if err := writeCategorizedLineStyle(paths.TransferToPlantFlowsStyle, 0.9, catalog.stylesForType(models.ObjectTypePlant)); err != nil {
		return fmt.Errorf("write transfer flow style: %w", err)
	}
	return nil
}

func writeCategorizedMarkerStyle(path string, shape string, sizeMM float64, styles []symbolStyle) error {
	var builder strings.Builder
	writeStyleHeader(&builder)
	writeCategories(&builder, styles)
	builder.WriteString("    <symbols>\n")
	for index, style := range styles {
		writeMarkerSymbol(&builder, index, shape, sizeMM, style.Color)
	}
	builder.WriteString("    </symbols>\n")
	writeStyleFooter(&builder)
	return writeStyleFile(path, builder.String())
}

func writeCategorizedLineStyle(path string, widthMM float64, styles []symbolStyle) error {
	var builder strings.Builder
	writeStyleHeader(&builder)
	writeCategories(&builder, styles)
	builder.WriteString("    <symbols>\n")
	for index, style := range styles {
		writeLineSymbol(&builder, index, widthMM, style.Color)
	}
	builder.WriteString("    </symbols>\n")
	writeStyleFooter(&builder)
	return writeStyleFile(path, builder.String())
}

func writeStyleHeader(builder *strings.Builder) {
	builder.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	builder.WriteString("<qgis version=\"3.28\" styleCategories=\"Symbology\">\n")
	builder.WriteString("  <renderer-v2 type=\"categorizedSymbol\" attr=\"style_key\" symbollevels=\"0\" enableorderby=\"0\" forceraster=\"0\">\n")
}

func writeCategories(builder *strings.Builder, styles []symbolStyle) {
	builder.WriteString("    <categories>\n")
	for index, style := range styles {
		fmt.Fprintf(
			builder,
			"      <category render=\"true\" symbol=\"%d\" value=\"%s\" label=\"%s\"/>\n",
			index,
			escapeXML(style.Key),
			escapeXML(style.Label),
		)
	}
	builder.WriteString("    </categories>\n")
}

func writeMarkerSymbol(builder *strings.Builder, index int, shape string, sizeMM float64, color string) {
	rgba := qgisRGBA(color, 255)
	fmt.Fprintf(builder, "      <symbol alpha=\"1\" clip_to_extent=\"1\" name=\"%d\" type=\"marker\">\n", index)
	builder.WriteString("        <layer class=\"SimpleMarker\" enabled=\"1\" locked=\"0\" pass=\"0\">\n")
	builder.WriteString("          <prop k=\"angle\" v=\"0\"/>\n")
	fmt.Fprintf(builder, "          <prop k=\"color\" v=\"%s\"/>\n", rgba)
	builder.WriteString("          <prop k=\"horizontal_anchor_point\" v=\"1\"/>\n")
	builder.WriteString("          <prop k=\"joinstyle\" v=\"bevel\"/>\n")
	fmt.Fprintf(builder, "          <prop k=\"name\" v=\"%s\"/>\n", escapeXML(shape))
	builder.WriteString("          <prop k=\"offset\" v=\"0,0\"/>\n")
	builder.WriteString("          <prop k=\"offset_unit\" v=\"MM\"/>\n")
	builder.WriteString("          <prop k=\"outline_color\" v=\"35,35,35,255\"/>\n")
	builder.WriteString("          <prop k=\"outline_style\" v=\"solid\"/>\n")
	builder.WriteString("          <prop k=\"outline_width\" v=\"0.15\"/>\n")
	builder.WriteString("          <prop k=\"outline_width_unit\" v=\"MM\"/>\n")
	builder.WriteString("          <prop k=\"scale_method\" v=\"diameter\"/>\n")
	fmt.Fprintf(builder, "          <prop k=\"size\" v=\"%.2f\"/>\n", sizeMM)
	builder.WriteString("          <prop k=\"size_unit\" v=\"MM\"/>\n")
	builder.WriteString("          <prop k=\"vertical_anchor_point\" v=\"1\"/>\n")
	builder.WriteString("        </layer>\n")
	builder.WriteString("      </symbol>\n")
}

func writeLineSymbol(builder *strings.Builder, index int, widthMM float64, color string) {
	rgba := qgisRGBA(color, 190)
	fmt.Fprintf(builder, "      <symbol alpha=\"1\" clip_to_extent=\"1\" name=\"%d\" type=\"line\">\n", index)
	builder.WriteString("        <layer class=\"SimpleLine\" enabled=\"1\" locked=\"0\" pass=\"0\">\n")
	builder.WriteString("          <prop k=\"capstyle\" v=\"round\"/>\n")
	builder.WriteString("          <prop k=\"customdash\" v=\"5;2\"/>\n")
	builder.WriteString("          <prop k=\"customdash_unit\" v=\"MM\"/>\n")
	builder.WriteString("          <prop k=\"joinstyle\" v=\"round\"/>\n")
	fmt.Fprintf(builder, "          <prop k=\"line_color\" v=\"%s\"/>\n", rgba)
	builder.WriteString("          <prop k=\"line_style\" v=\"solid\"/>\n")
	fmt.Fprintf(builder, "          <prop k=\"line_width\" v=\"%.2f\"/>\n", widthMM)
	builder.WriteString("          <prop k=\"line_width_unit\" v=\"MM\"/>\n")
	builder.WriteString("          <prop k=\"offset\" v=\"0\"/>\n")
	builder.WriteString("          <prop k=\"offset_unit\" v=\"MM\"/>\n")
	builder.WriteString("        </layer>\n")
	builder.WriteString("      </symbol>\n")
}

func writeStyleFooter(builder *strings.Builder) {
	builder.WriteString("  </renderer-v2>\n")
	builder.WriteString("</qgis>\n")
}

func writeStyleFile(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func qgisRGBA(hexColor string, alpha int) string {
	color := strings.TrimPrefix(strings.TrimSpace(hexColor), "#")
	if len(color) != 6 {
		return fmt.Sprintf("158,158,158,%d", alpha)
	}

	r, errR := strconv.ParseUint(color[0:2], 16, 8)
	g, errG := strconv.ParseUint(color[2:4], 16, 8)
	b, errB := strconv.ParseUint(color[4:6], 16, 8)
	if errR != nil || errG != nil || errB != nil {
		return fmt.Sprintf("158,158,158,%d", alpha)
	}
	return fmt.Sprintf("%d,%d,%d,%d", r, g, b, alpha)
}

func escapeXML(value string) string {
	return html.EscapeString(value)
}
