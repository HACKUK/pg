package orm

import "gopkg.in/pg.v4/types"

type join struct {
	BaseModel  tableModel
	JoinModel  tableModel
	Rel        *Relation
	ApplyQuery func(*Query) *Query

	Columns []string
}

func (j *join) JoinHasOne(q *Query) {
	var cond types.Q
	for _, pk := range j.Rel.Join.PKs {
		cond = appendSep(cond, " AND ")
		cond = q.FormatQuery(
			cond,
			`?.? = ?.?`,
			j.Rel.Field.ColName, pk.ColName,
			j.BaseModel.Table().Alias, types.F(j.Rel.Field.SQLName+"_"+pk.SQLName),
		)
	}

	q = q.Join(
		"LEFT JOIN ? AS ? ON ?",
		j.JoinModel.Table().Name, j.Rel.Field.ColName, cond,
	)
	q.columns = j.appendJoinedColumns(q.columns)
}

func (j *join) JoinBelongsTo(q *Query) {
	baseTable := j.BaseModel.Table()
	var cond types.Q
	for i, fk := range j.Rel.FKs {
		cond = appendSep(cond, " AND ")
		cond = q.FormatQuery(
			cond,
			`?.? = ?.?`,
			j.Rel.Field.ColName, fk.ColName,
			baseTable.Alias, baseTable.PKs[i].ColName,
		)
	}

	q = q.Join(
		"LEFT JOIN ? AS ? ON ?",
		j.JoinModel.Table().Name, j.Rel.Field.ColName, cond,
	)
	q.columns = j.appendJoinedColumns(q.columns)
}

func (j *join) Select(db dber) error {
	switch j.Rel.Type {
	case HasManyRelation:
		return j.selectMany(db)
	case Many2ManyRelation:
		return j.selectM2M(db)
	}
	panic("not reached")
}

func (j *join) selectMany(db dber) error {
	root := j.JoinModel.Root()
	index := j.JoinModel.Index()
	index = index[:len(index)-1]

	manyModel := newManyModel(j)
	q := NewQuery(db, manyModel)
	if j.ApplyQuery != nil {
		q = j.ApplyQuery(q)
	}

	q.columns = j.appendColumnsMany(q.columns)

	cols := columns(j.JoinModel.Table().Alias, "", j.Rel.FKs)
	vals := values(root, index, j.BaseModel.Table().PKs)
	q = q.Where(`(?) IN (?)`, types.Q(cols), types.Q(vals))

	if j.Rel.Polymorphic {
		q = q.Where(`? = ?`, types.F(j.Rel.BasePrefix+"type"), j.BaseModel.Table().ModelName)
	}

	err := q.Select()
	if err != nil {
		return err
	}

	return nil
}

func (j *join) selectM2M(db dber) error {
	index := j.JoinModel.Index()
	index = index[:len(index)-1]

	baseTable := j.BaseModel.Table()
	m2mCols := columns(j.Rel.M2MTableName, j.Rel.BasePrefix, baseTable.PKs)
	m2mVals := values(j.BaseModel.Root(), index, baseTable.PKs)

	m2mModel := newM2MModel(j)
	q := NewQuery(db, m2mModel)

	// Select all m2m intermediate table columns.
	q.columns = appendSep(q.columns, ", ")
	q.columns = append(q.columns, j.Rel.M2MTableName...)
	q.columns = append(q.columns, ".*"...)

	q.columns = j.appendColumnsMany(q.columns)

	q = q.Join(
		"JOIN ? ON (?) IN (?)",
		j.Rel.M2MTableName,
		types.Q(m2mCols), types.Q(m2mVals),
	)

	joinAlias := j.JoinModel.Table().Alias
	for _, pk := range j.JoinModel.Table().PKs {
		q = q.Where(
			"?.? = ?.?",
			joinAlias, pk.ColName,
			j.Rel.M2MTableName, types.F(j.Rel.JoinPrefix+pk.SQLName),
		)
	}

	err := q.Select()
	if err != nil {
		return err
	}

	return nil
}

func (j *join) appendJoinedColumns(dst []byte) []byte {
	var alias []byte
	alias = append(alias, j.Rel.Field.SQLName...)
	alias = append(alias, "__"...)

	if len(j.Columns) == 0 {
		for _, f := range j.JoinModel.Table().Fields {
			dst = appendSep(dst, ", ")
			dst = append(dst, j.Rel.Field.ColName...)
			dst = append(dst, '.')
			dst = append(dst, f.ColName...)
			dst = append(dst, " AS "...)
			columnAlias := append(alias, f.SQLName...)
			dst = types.AppendFieldBytes(dst, columnAlias, 1)
			alias = columnAlias[:len(alias)]
		}
		return dst
	}

	for _, column := range j.Columns {
		if column == "_" {
			continue
		}

		dst = appendSep(dst, ", ")
		dst = append(dst, j.Rel.Field.ColName...)
		dst = append(dst, '.')
		dst = types.AppendField(dst, column, 1)
		dst = append(dst, " AS "...)
		columnAlias := append(alias, column...)
		dst = types.AppendFieldBytes(dst, append(alias, column...), 1)
		alias = columnAlias[:len(alias)]
	}
	return dst
}

func (j *join) appendColumnsMany(dst []byte) []byte {
	if len(j.Columns) == 0 {
		dst = appendSep(dst, ", ")
		dst = append(dst, j.JoinModel.Table().Alias...)
		dst = append(dst, ".*"...)
		return dst
	}

	for _, column := range j.Columns {
		if column == "_" {
			continue
		}

		dst = appendSep(dst, ", ")
		dst = append(dst, j.JoinModel.Table().Alias...)
		dst = append(dst, '.')
		dst = types.AppendField(dst, column, 1)
	}
	return dst
}
