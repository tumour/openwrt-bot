package network

import (
	"context"
	"fmt"
)

// RunSpeedTest — use case "замерить скорость канала". Тонкий (прокси к одному
// порту), но существует отдельной единицей: рядом можно будет добавить
// throttling ("не чаще раза в минуту") / кеш последнего замера без правок adapter.
type RunSpeedTest struct {
	st SpeedTestPort
}

func NewRunSpeedTest(st SpeedTestPort) *RunSpeedTest {
	return &RunSpeedTest{st: st}
}

type (
	RunSpeedTestInput  struct{}
	RunSpeedTestOutput struct {
		Result SpeedResult
	}
)

func (uc *RunSpeedTest) Execute(ctx context.Context, _ RunSpeedTestInput) (RunSpeedTestOutput, error) {
	r, err := uc.st.Measure(ctx)
	if err != nil {
		return RunSpeedTestOutput{}, fmt.Errorf("measure speed: %w", err)
	}
	return RunSpeedTestOutput{Result: r}, nil
}
