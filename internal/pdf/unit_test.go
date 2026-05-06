package pdf

import "testing"

func TestSetPageSize(t *testing.T) {
	d := NewDocument()
	d.SetPageSize(Letter)
	if d.pageSize != Letter {
		t.Errorf("SetPageSize: got %v, want %v", d.pageSize, Letter)
	}
	d.SetPageSize(A4)
	if d.pageSize != A4 {
		t.Errorf("SetPageSize A4: got %v", d.pageSize)
	}
}

func TestSetMargins(t *testing.T) {
	d := NewDocument()
	p := d.AddPage()
	p.SetMargins(10, 20, 30, 40)
	want := Margins{Top: 10, Right: 20, Bottom: 30, Left: 40}
	if p.margins != want {
		t.Errorf("SetMargins: got %+v, want %+v", p.margins, want)
	}
}

func TestTableRendererTotalHeight(t *testing.T) {
	tr := newTableRenderer([]string{"a", "b"}, [][]string{{"1", "2"}, {"3", "4"}}, 400)
	got := tr.totalHeight()
	want := tr.rowHeight * 3 // 1 header + 2 rows
	if got != want {
		t.Errorf("totalHeight = %v, want %v", got, want)
	}
}

func TestStreamBuilderWriteS(t *testing.T) {
	sb := &streamBuilder{}
	sb.writeS("hello ")
	sb.writeS("world")
	if string(sb.data) != "hello world" {
		t.Errorf("writeS: got %q", string(sb.data))
	}
}
