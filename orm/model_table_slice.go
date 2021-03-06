package orm

import "reflect"

type sliceTableModel struct {
	structTableModel

	slice      reflect.Value
	sliceOfPtr bool
	zeroElem   reflect.Value
}

var _ tableModel = (*sliceTableModel)(nil)

func (m *sliceTableModel) init(sliceType reflect.Type) {
	m.sliceOfPtr = sliceType.Elem().Kind() == reflect.Ptr
	if !m.sliceOfPtr {
		m.zeroElem = reflect.Zero(m.table.Type)
	}
}

func (sliceTableModel) useQueryOne() bool {
	return false
}

func (m *sliceTableModel) Join(name string, apply func(*Query) *Query) *join {
	return m.join(m.Value(), name, apply)
}

func (m *sliceTableModel) Bind(bind reflect.Value) {
	m.slice = bind.Field(m.index[len(m.index)-1])
}

func (m *sliceTableModel) Value() reflect.Value {
	return m.slice
}

func (m *sliceTableModel) NewModel() ColumnScanner {
	if !m.strct.IsValid() {
		m.slice.Set(m.slice.Slice(0, 0))
	}

	m.strct = m.nextElem()
	m.structTableModel.NewModel()
	return m
}

func (m *sliceTableModel) nextElem() reflect.Value {
	if m.slice.Len() < m.slice.Cap() {
		m.slice.Set(m.slice.Slice(0, m.slice.Len()+1))
		return m.slice.Index(m.slice.Len() - 1)
	}

	if m.sliceOfPtr {
		elem := reflect.New(m.table.Type)
		m.slice.Set(reflect.Append(m.slice, elem))
		return elem.Elem()
	}

	m.slice.Set(reflect.Append(m.slice, m.zeroElem))
	return m.slice.Index(m.slice.Len() - 1)
}
