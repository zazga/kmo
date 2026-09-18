package app

import "testing"

func TestSidebarWindowStartKeepsSelectionVisible(t *testing.T) {
	tests := []struct {
		name       string
		selected   int
		total      int
		visible    int
		wantStart  int
	}{
		{name: "fits without scrolling", selected: 4, total: 8, visible: 10, wantStart: 0},
		{name: "scrolls down when selection leaves viewport", selected: 10, total: 20, visible: 10, wantStart: 1},
		{name: "scrolls to final page", selected: 19, total: 20, visible: 10, wantStart: 10},
		{name: "clamps negative selection", selected: -1, total: 20, visible: 10, wantStart: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sidebarWindowStart(tt.selected, tt.total, tt.visible); got != tt.wantStart {
				t.Fatalf("sidebarWindowStart(%d,%d,%d) = %d, want %d", tt.selected, tt.total, tt.visible, got, tt.wantStart)
			}
		})
	}
}
