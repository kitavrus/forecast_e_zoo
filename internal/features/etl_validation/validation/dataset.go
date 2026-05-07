package validation

// Row — обобщённое представление одной строки entity (column → value).
//
// Используется как in-memory представление до загрузки в pg_temp.stg_*.
// Транспорт между transformer-ом и engine: transformer декодирует
// JSON-bundle Modul 1 и заполняет Dataset.
type Row map[string]any

// Dataset — набор entity → list of rows.
//
// Семантика «снимок данных одного ETL run, доступный валидаторам».
// Engine не модифицирует Dataset.
type Dataset struct {
	rows map[string][]Row
}

// NewDataset — пустой Dataset.
func NewDataset() *Dataset {
	return &Dataset{rows: make(map[string][]Row)}
}

// Add добавляет строку в entity.
func (d *Dataset) Add(entity string, row Row) {
	d.rows[entity] = append(d.rows[entity], row)
}

// SetEntity полностью заменяет rows для entity.
func (d *Dataset) SetEntity(entity string, rows []Row) {
	cp := make([]Row, len(rows))
	copy(cp, rows)
	d.rows[entity] = cp
}

// Entities возвращает список имён загруженных entity.
func (d *Dataset) Entities() []string {
	out := make([]string, 0, len(d.rows))
	for k := range d.rows {
		out = append(out, k)
	}
	return out
}

// Rows возвращает строки entity (или nil, если entity отсутствует).
func (d *Dataset) Rows(entity string) []Row { return d.rows[entity] }

// CountAll — сумма количества строк по всем entity.
func (d *Dataset) CountAll() int {
	n := 0
	for _, rows := range d.rows {
		n += len(rows)
	}
	return n
}
