package simulator

import (
	"log"
	"sync"
	"time"
)

func Backfill(st *SimulatorState, clock *Clock, emit func(TelemetryEvent)) {
	log.Printf("[simulator] backfilling %d devices...", len(st.Devices))

	simNow := clock.NowSim()

	var wg sync.WaitGroup
	for _, dev := range st.Devices {
		wg.Add(1)
		go func(d *DeviceState) {
			defer wg.Done()
			d.Seq = 0
			d.Seq++
			emit(TelemetryEvent{
				DeviceID:  d.DeviceID,
				PoleID:    d.PoleID,
				Event:     "boot",
				Energized: false,
				Ts:        clock.TsForSim(simNow, d.ClockSkewSecs),
				Seq:       d.Seq,
				BatteryMV: d.BatteryMV,
				RSSI:      d.RSSI,
				Fw:        d.Firmware,
			})
			time.Sleep(5 * time.Millisecond)
			d.Seq++
			emit(TelemetryEvent{
				DeviceID:  d.DeviceID,
				PoleID:    d.PoleID,
				Event:     "power_restored",
				Energized: true,
				Ts:        clock.TsForSim(simNow+1, d.ClockSkewSecs),
				Seq:       d.Seq,
				BatteryMV: d.BatteryMV,
				RSSI:      d.RSSI,
				Fw:        d.Firmware,
			})
		}(dev)
	}

	wg.Wait()
	log.Printf("[simulator] backfill complete")
}
