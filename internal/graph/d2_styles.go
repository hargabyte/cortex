package graph

// D2 Visual Design System
// ========================
// Professional styling for CX-generated D2 diagrams.
// See d2_design_system.d2 for the complete D2 reference implementation.

// D2Theme contains theme configuration for D2 diagrams.
type D2Theme struct {
	ID           int    // D2 theme ID (e.g., 200 for Mixed Berry Blue)
	Name         string // Human-readable name
	LayoutEngine string // Layout engine: dagre, elk, tala
}

// D2Themes available for diagram generation.
// See: d2 themes (CLI) or https://d2lang.com/tour/themes/
var D2Themes = map[string]D2Theme{
	"default":          {ID: 8, Name: "Colorblind Clear", LayoutEngine: "elk"},
	"colorblind-clear": {ID: 8, Name: "Colorblind Clear", LayoutEngine: "elk"},
	"vanilla-nitro":    {ID: 100, Name: "Vanilla Nitro Cola", LayoutEngine: "elk"},
	"mixed-berry":      {ID: 5, Name: "Mixed Berry Blue", LayoutEngine: "elk"},
	"grape-soda":       {ID: 6, Name: "Grape Soda", LayoutEngine: "elk"},
	"earth-tones":      {ID: 103, Name: "Earth Tones", LayoutEngine: "elk"},
	"terminal":         {ID: 300, Name: "Terminal", LayoutEngine: "elk"},
	"dark":             {ID: 200, Name: "Dark Mauve", LayoutEngine: "elk"},
	"dark-flagship":    {ID: 201, Name: "Dark Flagship Terrastruct", LayoutEngine: "elk"},
	"neutral":          {ID: 0, Name: "Neutral Default", LayoutEngine: "elk"},
}

// D2Color represents a color with fill and stroke values.
type D2Color struct {
	Fill   string // Background fill color (hex)
	Stroke string // Border/stroke color (hex)
}

// D2EntityColors maps entity types to their colors.
var D2EntityColors = map[string]D2Color{
	"function":  {Fill: "#e3f2fd", Stroke: "#1976d2"}, // Light blue
	"method":    {Fill: "#e3f2fd", Stroke: "#1976d2"}, // Light blue
	"type":      {Fill: "#f3e5f5", Stroke: "#7b1fa2"}, // Light purple
	"struct":    {Fill: "#f3e5f5", Stroke: "#7b1fa2"}, // Light purple
	"class":     {Fill: "#f3e5f5", Stroke: "#7b1fa2"}, // Light purple
	"interface": {Fill: "#fff3e0", Stroke: "#f57c00"}, // Light orange
	"constant":  {Fill: "#e8f5e9", Stroke: "#388e3c"}, // Light green
	"variable":  {Fill: "#e8f5e9", Stroke: "#388e3c"}, // Light green
	"enum":      {Fill: "#e8f5e9", Stroke: "#388e3c"}, // Light green
	"database":  {Fill: "#eceff1", Stroke: "#455a64"}, // Light gray
	"storage":   {Fill: "#eceff1", Stroke: "#455a64"}, // Light gray
	"http":      {Fill: "#e0f7fa", Stroke: "#0097a7"}, // Light cyan
	"handler":   {Fill: "#e0f7fa", Stroke: "#0097a7"}, // Light cyan
	"test":      {Fill: "#e8f5e9", Stroke: "#388e3c"}, // Light green
	"package":   {Fill: "#fafafa", Stroke: "#9e9e9e"}, // Near white
	"module":    {Fill: "#fafafa", Stroke: "#9e9e9e"}, // Near white
	"external":  {Fill: "#fafafa", Stroke: "#9e9e9e"}, // Near white
	"default":   {Fill: "#ffffff", Stroke: "#757575"}, // White/gray
}

// D2ImportanceColors maps importance levels to colors.
var D2ImportanceColors = map[string]D2Color{
	"keystone":    {Fill: "#fff3e0", Stroke: "#e65100"}, // Warm orange
	"bottleneck":  {Fill: "#fff8e1", Stroke: "#ff8f00"}, // Amber
	"high-fan-in": {Fill: "#e3f2fd", Stroke: "#1565c0"}, // Blue
	"high-fan-out":{Fill: "#fce4ec", Stroke: "#c2185b"}, // Pink
	"normal":      {Fill: "#ffffff", Stroke: "#757575"}, // White/gray
	"leaf":        {Fill: "#fafafa", Stroke: "#bdbdbd"}, // Off white
}

// D2CoverageColors maps coverage levels to colors.
var D2CoverageColors = map[string]D2Color{
	"high":   {Fill: "#c8e6c9", Stroke: "#4caf50"}, // Green (>80%)
	"medium": {Fill: "#fff9c4", Stroke: "#fbc02d"}, // Yellow (50-80%)
	"low":    {Fill: "#ffcdd2", Stroke: "#f44336"}, // Red (<50%)
	"none":   {Fill: "#f5f5f5", Stroke: "#9e9e9e"}, // Gray (0%)
}

// D2RiskColors maps risk levels to colors.
var D2RiskColors = map[string]D2Color{
	"critical": {Fill: "#ffebee", Stroke: "#c62828"},
	"warning":  {Fill: "#fff8e1", Stroke: "#f57f17"},
	"info":     {Fill: "#e3f2fd", Stroke: "#1976d2"},
	"ok":       {Fill: "#e8f5e9", Stroke: "#388e3c"},
}

// D2ChangeColors maps change states to colors for before/after diagrams.
var D2ChangeColors = map[string]D2Color{
	"added":     {Fill: "#c8e6c9", Stroke: "#2e7d32"}, // Green - new entities
	"modified":  {Fill: "#fff9c4", Stroke: "#f9a825"}, // Yellow - changed entities
	"deleted":   {Fill: "#ffcdd2", Stroke: "#c62828"}, // Red - removed entities
	"unchanged": {Fill: "#ffffff", Stroke: "#757575"}, // White/gray - no change
}

// D2LayerColors maps architectural layers to colors.
var D2LayerColors = map[string]D2Color{
	"api":     {Fill: "#e0f7fa", Stroke: "#00838f"}, // Cyan - API/HTTP layer
	"service": {Fill: "#e3f2fd", Stroke: "#1565c0"}, // Blue - Business logic
	"data":    {Fill: "#eceff1", Stroke: "#455a64"}, // Gray - Data/storage
	"domain":  {Fill: "#f3e5f5", Stroke: "#6a1b9a"}, // Purple - Domain models
	"default": {Fill: "#fafafa", Stroke: "#e0e0e0"}, // Light gray
}

// D2Icon represents an icon URL for D2 diagrams.
type D2Icon string

// D2EntityIcons maps entity types to icon URLs from icons.terrastruct.com.
var D2EntityIcons = map[string]D2Icon{
	"function":  "https://icons.terrastruct.com/essentials%2F142-lightning.svg",
	"method":    "https://icons.terrastruct.com/essentials%2F009-gear.svg",
	"type":      "https://icons.terrastruct.com/essentials%2F108-box.svg",
	"struct":    "https://icons.terrastruct.com/essentials%2F108-box.svg",
	"class":     "https://icons.terrastruct.com/essentials%2F108-box.svg",
	"interface": "https://icons.terrastruct.com/essentials%2F092-plug.svg",
	"constant":  "https://icons.terrastruct.com/essentials%2F078-pin.svg",
	"variable":  "https://icons.terrastruct.com/essentials%2F078-pin.svg",
	"enum":      "https://icons.terrastruct.com/essentials%2F113-list.svg",
	"database":  "https://icons.terrastruct.com/essentials%2F119-database.svg",
	"storage":   "https://icons.terrastruct.com/essentials%2F119-database.svg",
	"http":      "https://icons.terrastruct.com/essentials%2F140-earth.svg",
	"handler":   "https://icons.terrastruct.com/essentials%2F140-earth.svg",
	"test":      "https://icons.terrastruct.com/essentials%2F134-checkmark.svg",
	"package":   "https://icons.terrastruct.com/essentials%2F106-archive.svg",
	"module":    "https://icons.terrastruct.com/essentials%2F106-archive.svg",
}

// D2LanguageIcons maps programming languages to icon URLs.
var D2LanguageIcons = map[string]D2Icon{
	"go":         "https://icons.terrastruct.com/dev%2Fgo.svg",
	"typescript": "https://icons.terrastruct.com/dev%2Ftypescript.svg",
	"javascript": "https://icons.terrastruct.com/dev%2Fjavascript.svg",
	"python":     "https://icons.terrastruct.com/dev%2Fpython.svg",
	"java":       "https://icons.terrastruct.com/dev%2Fjava.svg",
	"rust":       "https://icons.terrastruct.com/dev%2Frustlang.svg",
	"c":          "https://icons.terrastruct.com/dev%2Fc.svg",
	"cpp":        "https://icons.terrastruct.com/dev%2Fcplusplus.svg",
	"csharp":     "https://icons.terrastruct.com/dev%2Fcsharp.svg",
	"php":        "https://icons.terrastruct.com/dev%2Fphp.svg",
	"ruby":       "https://icons.terrastruct.com/dev%2Fruby.svg",
	"kotlin":     "https://icons.terrastruct.com/dev%2Fkotlin.svg",
}

// D2StatusIcons for various status indicators.
var D2StatusIcons = map[string]D2Icon{
	"warning":  "https://icons.terrastruct.com/essentials%2F149-warning-2.svg",
	"error":    "https://icons.terrastruct.com/essentials%2F150-error-1.svg",
	"info":     "https://icons.terrastruct.com/essentials%2F152-info-1.svg",
	"success":  "https://icons.terrastruct.com/essentials%2F134-checkmark.svg",
	"lock":     "https://icons.terrastruct.com/essentials%2F091-lock.svg",
	"server":   "https://icons.terrastruct.com/essentials%2F112-server.svg",
	"cloud":    "https://icons.terrastruct.com/essentials%2F096-cloud.svg",
	"network":  "https://icons.terrastruct.com/essentials%2F100-network.svg",
	"search":   "https://icons.terrastruct.com/essentials%2F107-zoom.svg",
	"display":  "https://icons.terrastruct.com/essentials%2F087-display.svg",
	"shield":   "https://icons.terrastruct.com/essentials%2F092-shield.svg",
}

// D2EdgeStyleDef defines the visual style for an edge/connection.
type D2EdgeStyleDef struct {
	Arrow       string // Arrow type: ->, <-, <->, --
	StrokeColor string // Stroke color (hex)
	StrokeWidth int    // Stroke width in pixels
	StrokeDash  int    // Dash pattern (0 for solid)
	Animated    bool   // Whether to animate the edge
}

// D2EdgeStyles maps dependency types to edge styles.
var D2EdgeStyles = map[string]D2EdgeStyleDef{
	"calls": {
		Arrow:       "->",
		StrokeColor: "#424242",
		StrokeWidth: 1,
		StrokeDash:  0,
		Animated:    false,
	},
	"uses_type": {
		Arrow:       "->",
		StrokeColor: "#757575",
		StrokeWidth: 1,
		StrokeDash:  3,
		Animated:    false,
	},
	"implements": {
		Arrow:       "->",
		StrokeColor: "#f57c00",
		StrokeWidth: 1,
		StrokeDash:  5,
		Animated:    false,
	},
	"extends": {
		Arrow:       "->",
		StrokeColor: "#7b1fa2",
		StrokeWidth: 2,
		StrokeDash:  0,
		Animated:    false,
	},
	"data_flow": {
		Arrow:       "->",
		StrokeColor: "#1976d2",
		StrokeWidth: 2,
		StrokeDash:  0,
		Animated:    true,
	},
	"imports": {
		Arrow:       "->",
		StrokeColor: "#9e9e9e",
		StrokeWidth: 1,
		StrokeDash:  2,
		Animated:    false,
	},
	"references": {
		Arrow:       "->",
		StrokeColor: "#9e9e9e",
		StrokeWidth: 1,
		StrokeDash:  3,
		Animated:    false,
	},
	"tests": {
		Arrow:       "->",
		StrokeColor: "#4caf50",
		StrokeWidth: 1,
		StrokeDash:  4,
		Animated:    false,
	},
	"default": {
		Arrow:       "->",
		StrokeColor: "#757575",
		StrokeWidth: 1,
		StrokeDash:  0,
		Animated:    false,
	},
}

// D2NodeStyle contains all styling properties for a D2 node.
type D2NodeStyle struct {
	Shape        string  // D2 shape (rectangle, cylinder, etc.)
	Fill         string  // Background color
	Stroke       string  // Border color
	StrokeWidth  int     // Border width
	StrokeDash   int     // Border dash pattern (0 for solid)
	BorderRadius int     // Corner radius
	Shadow       bool    // Drop shadow
	Opacity      float64 // Opacity (0.0-1.0)
	Icon         string  // Icon URL (optional)
}

// GetD2NodeStyle builds a complete node style from entity properties.
func GetD2NodeStyle(entityType, importance string, coverage float64, language string) D2NodeStyle {
	style := D2NodeStyle{
		Shape:        "rectangle",
		StrokeWidth:  1,
		BorderRadius: 4,
		Opacity:      1.0,
	}

	// Base shape from entity type
	if shape, ok := EntityShapes[entityType]; ok {
		style.Shape = shape.D2Shape
	}

	// Colors from entity type
	if colors, ok := D2EntityColors[entityType]; ok {
		style.Fill = colors.Fill
		style.Stroke = colors.Stroke
	} else {
		style.Fill = D2EntityColors["default"].Fill
		style.Stroke = D2EntityColors["default"].Stroke
	}

	// Override with importance colors if not normal
	if importance != "" && importance != "normal" {
		if colors, ok := D2ImportanceColors[importance]; ok {
			style.Fill = colors.Fill
			style.Stroke = colors.Stroke
		}
		if importance == "keystone" {
			style.StrokeWidth = 3
			style.Shadow = true
		} else if importance == "bottleneck" || importance == "high-fan-in" || importance == "high-fan-out" {
			style.StrokeWidth = 2
		} else if importance == "leaf" {
			style.Opacity = 0.9
		}
	}

	// Apply coverage coloring if provided
	if coverage >= 0 {
		coverageColors := GetCoverageColor(coverage)
		// Blend coverage into stroke color for visual indicator
		style.Stroke = coverageColors.Stroke
		if coverage == 0 {
			style.StrokeDash = 3
		}
	}

	// Icon from entity type or language
	if icon, ok := D2EntityIcons[entityType]; ok {
		style.Icon = string(icon)
	}
	// Language icon can override entity icon for functions
	if language != "" {
		if langIcon, ok := D2LanguageIcons[language]; ok {
			style.Icon = string(langIcon)
		}
	}

	return style
}

// ApplyChangeStateStyle modifies a node style to reflect the change state.
// Change state colors override entity/importance colors for visual emphasis.
func ApplyChangeStateStyle(style *D2NodeStyle, changeState string) {
	if changeState == "" {
		return
	}

	if colors, ok := D2ChangeColors[changeState]; ok {
		style.Fill = colors.Fill
		style.Stroke = colors.Stroke
	}

	// Visual emphasis for changes
	switch changeState {
	case "added":
		style.StrokeWidth = 2
	case "modified":
		style.StrokeWidth = 2
		style.StrokeDash = 3 // Dashed border for modified
	case "deleted":
		style.StrokeWidth = 2
		style.Opacity = 0.7 // Slightly faded for deleted
	}
}

// GetCoverageColor returns the color for a coverage percentage.
func GetCoverageColor(coverage float64) D2Color {
	switch {
	case coverage >= 80:
		return D2CoverageColors["high"]
	case coverage >= 50:
		return D2CoverageColors["medium"]
	case coverage > 0:
		return D2CoverageColors["low"]
	default:
		return D2CoverageColors["none"]
	}
}

// GetCoverageLevel returns the coverage level string for a percentage.
func GetCoverageLevel(coverage float64) string {
	switch {
	case coverage >= 80:
		return "high"
	case coverage >= 50:
		return "medium"
	case coverage > 0:
		return "low"
	default:
		return "none"
	}
}

// GetD2EdgeStyle returns the edge style for a dependency type.
func GetD2EdgeStyle(depType string) D2EdgeStyleDef {
	if style, ok := D2EdgeStyles[depType]; ok {
		return style
	}
	return D2EdgeStyles["default"]
}

// GetD2Icon returns an icon URL for an entity type.
func GetD2Icon(entityType string) D2Icon {
	if icon, ok := D2EntityIcons[entityType]; ok {
		return icon
	}
	return ""
}

// GetD2LanguageIcon returns an icon URL for a programming language.
func GetD2LanguageIcon(language string) D2Icon {
	if icon, ok := D2LanguageIcons[language]; ok {
		return icon
	}
	return ""
}

// GetD2StatusIcon returns an icon URL for a status indicator.
func GetD2StatusIcon(status string) D2Icon {
	if icon, ok := D2StatusIcons[status]; ok {
		return icon
	}
	return ""
}

// GetD2LayerColor returns the color for an architectural layer.
func GetD2LayerColor(layer string) D2Color {
	if color, ok := D2LayerColors[layer]; ok {
		return color
	}
	return D2LayerColors["default"]
}

// D2StyleToString converts a D2NodeStyle to D2 style block syntax.
func D2StyleToString(style D2NodeStyle) string {
	var parts []string

	if style.Fill != "" {
		parts = append(parts, "fill: \""+style.Fill+"\"")
	}
	if style.Stroke != "" {
		parts = append(parts, "stroke: \""+style.Stroke+"\"")
	}
	if style.StrokeWidth > 0 {
		parts = append(parts, "stroke-width: "+itoa(style.StrokeWidth))
	}
	if style.StrokeDash > 0 {
		parts = append(parts, "stroke-dash: "+itoa(style.StrokeDash))
	}
	if style.BorderRadius > 0 {
		parts = append(parts, "border-radius: "+itoa(style.BorderRadius))
	}
	if style.Shadow {
		parts = append(parts, "shadow: true")
	}
	if style.Opacity > 0 && style.Opacity < 1.0 {
		parts = append(parts, "opacity: "+ftoa(style.Opacity))
	}

	if len(parts) == 0 {
		return ""
	}

	result := "style: {\n"
	for _, part := range parts {
		result += "    " + part + "\n"
	}
	result += "  }"
	return result
}

// D2EdgeStyleToString converts a D2EdgeStyleDef to D2 style syntax.
func D2EdgeStyleToString(style D2EdgeStyleDef) string {
	var parts []string

	if style.StrokeColor != "" {
		parts = append(parts, "stroke: \""+style.StrokeColor+"\"")
	}
	if style.StrokeWidth > 0 && style.StrokeWidth != 1 {
		parts = append(parts, "stroke-width: "+itoa(style.StrokeWidth))
	}
	if style.StrokeDash > 0 {
		parts = append(parts, "stroke-dash: "+itoa(style.StrokeDash))
	}
	if style.Animated {
		parts = append(parts, "animated: true")
	}

	if len(parts) == 0 {
		return ""
	}

	result := "style: {\n"
	for _, part := range parts {
		result += "    " + part + "\n"
	}
	result += "  }"
	return result
}

// Helper functions for string conversion
func itoa(i int) string {
	if i < 0 {
		return "-" + itoa(-i)
	}
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}

func ftoa(f float64) string {
	// Simple float formatting for common values
	switch {
	case f >= 0.95:
		return "1.0"
	case f <= 0.05:
		return "0.0"
	case f >= 0.85:
		return "0.9"
	case f >= 0.75:
		return "0.8"
	case f >= 0.65:
		return "0.7"
	case f >= 0.55:
		return "0.6"
	case f >= 0.45:
		return "0.5"
	case f >= 0.35:
		return "0.4"
	case f >= 0.25:
		return "0.3"
	case f >= 0.15:
		return "0.2"
	default:
		return "0.1"
	}
}
