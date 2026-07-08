package domain

import "strconv"

// DeviceJob — отложенная операция над устройством: что (Action) над кем (MAC).
// Это единица, которую таймерный движок носит как непрозрачную задачу J и отдаёт
// Runner'у при срабатывании (composition root дёргает соответствующий use case
// device.*). Плоский набор value-объектов — инварианты уже в MAC и Action,
// перепроверять нечего.
type DeviceJob struct {
	MAC    MAC
	Action Action
}

// TaskID — идентификатор запланированной задачи, уникальный в пределах процесса
// (присваивает планировщик). Ходит в payload кнопки отмены.
type TaskID uint64

// String реализует fmt.Stringer.
func (id TaskID) String() string { return strconv.FormatUint(uint64(id), 10) }
