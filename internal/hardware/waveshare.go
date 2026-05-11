package hardware

import (
	"encoding/binary"
	"fmt"
	"math"

	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/host/v3"
)

const (
	// Waveshare default I2C address
	INA219Address = 0x41

	// INA219 Registers
	RegConfig       = 0x00
	RegShuntVoltage = 0x01
	RegBusVoltage   = 0x02
	RegPower        = 0x03
	RegCurrent      = 0x04
	RegCalibration  = 0x05

	// Calibration values based on Waveshare's 16V 5A profile
	CalValue   uint16  = 26868
	ConfigVal  uint16  = 0x0EEF // RANGE_16V | DIV_2_80MV | 12BIT_32S | SANDBVOLT_CONTINUOUS
	CurrentLSB float64 = 0.1524
	PowerLSB   float64 = 0.003048
)

// UPS represents the Waveshare 3S UPS I2C connection
type UPS struct {
	dev *i2c.Dev
}

// NewUPS opens the I2C bus, connects to the INA219, and pushes the calibration profile.
func NewUPS() (*UPS, error) {
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize periph.io: %w", err)
	}

	bus, err := i2creg.Open("")
	if err != nil {
		return nil, fmt.Errorf("failed to open I2C bus: %w", err)
	}

	dev := &i2c.Dev{Addr: INA219Address, Bus: bus}
	ups := &UPS{dev: dev}

	if err := ups.calibrate(); err != nil {
		return nil, fmt.Errorf("failed to calibrate INA219: %w", err)
	}

	return ups, nil
}

// readReg reads a 16-bit register from the INA219.
func (u *UPS) readReg(reg byte) (uint16, error) {
	write := []byte{reg}
	read := make([]byte, 2)

	if err := u.dev.Tx(write, read); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(read), nil
}

// writeReg writes a 16-bit value to a specific INA219 register.
func (u *UPS) writeReg(reg byte, value uint16) error {
	data := make([]byte, 3)
	data[0] = reg
	binary.BigEndian.PutUint16(data[1:], value)

	return u.dev.Tx(data, nil)
}

// calibrate pushes the manufacturer's configuration and calibration values to the chip.
func (u *UPS) calibrate() error {
	if err := u.writeReg(RegCalibration, CalValue); err != nil {
		return err
	}
	return u.writeReg(RegConfig, ConfigVal)
}

// ReadVoltage returns the current bus voltage (Load Voltage).
func (u *UPS) ReadVoltage() (float64, error) {
	// Re-assert calibration before reading, as per manufacturer code pattern
	_ = u.writeReg(RegCalibration, CalValue)

	val, err := u.readReg(RegBusVoltage)
	if err != nil {
		return 0, err
	}

	// Shift right 3 bits and multiply by 4mV LSB
	return float64(val>>3) * 0.004, nil
}

// ReadCurrent returns the current draw in milliamps (mA).
func (u *UPS) ReadCurrent() (float64, error) {
	val, err := u.readReg(RegCurrent)
	if err != nil {
		return 0, err
	}

	// Handle 16-bit two's complement for negative values
	intVal := int16(val)
	return float64(intVal) * CurrentLSB, nil
}

// ReadPower returns the power consumption in Watts (W).
func (u *UPS) ReadPower() (float64, error) {
	_ = u.writeReg(RegCalibration, CalValue)

	val, err := u.readReg(RegPower)
	if err != nil {
		return 0, err
	}

	intVal := int16(val)
	return float64(intVal) * PowerLSB, nil
}

// GetBatteryPercentage calculates the remaining capacity (0-100%).
func (u *UPS) GetBatteryPercentage() (float64, error) {
	voltage, err := u.ReadVoltage()
	if err != nil {
		return 0, err
	}

	// Manufacturer's 3S voltage calculation
	percentage := ((voltage - 9.0) / 3.6) * 100.0

	// Clamp the value so it never reads below 0% or above 100%
	percentage = math.Max(0, math.Min(100, percentage))

	return percentage, nil
}

func (u *UPS) IsCharging() (bool, error) {
	current, err := u.ReadCurrent()
	if err != nil {
		return false, err
	}

	// A positive current means the battery is actively charging
	// >10mA to filter out tiny electrical noise around 0
	if current > 10.0 {
		return true, nil
	}

	// Edge Case: If the battery is 100% full, the charging current drops to ~0mA.
	// If current is near zero but the voltage is at the wall-power peak (>12.4V),
	// it is still plugged into the wall.
	voltage, _ := u.ReadVoltage()
	if current > -50.0 && voltage > 12.4 {
		return true, nil
	}

	// Negative current means it is discharging (running on battery).
	return false, nil
}
