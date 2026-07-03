package jsonview

// LineKind identifies what kind of row a Line renders as.
type LineKind int

const (
	// LineValue is a self-contained line: a scalar, an empty container
	// ("{}"/"[]"), or a collapsed non-empty container ("{…3}"/"[…5]").
	LineValue LineKind = iota
	// LineOpen is the opening "{"/"[" of an expanded, non-empty container;
	// its children follow as subsequent Lines.
	LineOpen
	// LineClose is the closing "}"/"]" matching a LineOpen.
	LineClose
)

// Line is one visible row produced by Flatten.
type Line struct {
	Node   *Node
	Kind   LineKind
	Depth  int
	IsLast bool // false => this value needs a trailing comma when rendered
}

// Flatten walks the tree honoring each Node's Collapsed state and returns
// the currently-visible lines in display order. Call it again after
// changing a Node's Collapsed field to re-derive the visible lines.
func Flatten(root *Node) []Line {
	var lines []Line
	flattenInto(&lines, root, 0, true)
	return lines
}

func flattenInto(lines *[]Line, n *Node, depth int, isLast bool) {
	expandable := (n.Kind == KindObject || n.Kind == KindArray) && len(n.Children) > 0
	if expandable && !n.Collapsed {
		*lines = append(*lines, Line{Node: n, Kind: LineOpen, Depth: depth, IsLast: isLast})
		for i, c := range n.Children {
			flattenInto(lines, c, depth+1, i == len(n.Children)-1)
		}
		*lines = append(*lines, Line{Node: n, Kind: LineClose, Depth: depth, IsLast: isLast})
		return
	}
	*lines = append(*lines, Line{Node: n, Kind: LineValue, Depth: depth, IsLast: isLast})
}
