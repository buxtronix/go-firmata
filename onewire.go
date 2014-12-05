package firmata

import (
	"fmt"
)

// OneWireSubCommand is the command to send to the device.
type OneWireSubCommand byte

// OneWireAddress is a ROM address.
type OneWireAddress []byte

// A OneWireRequest is a Firmata OneWire request.
type OneWireRequest struct {
	// Command is the command to send to the Firmata firmware.
	Command OneWireSubCommand
	// Address is the ROM address for the 1wire device.
	Address OneWireAddress
	// ReadCount is the number of bytes to read from a device.
	ReadCount int32
	// CorrelationId is the unique ID for the request.
	CorrelationId int32
	// DelayMs is a delay to send prior to the command executing.
	DelayMs int32
	// Data is the data to send to the device.
	Data []byte
}

// Encode encodes the request to 7 bit data ready to send on the wire.
func (r *OneWireRequest) Encode() []byte {
	var d []byte
	if r.Command&OW_SELECT > 0 {
		d = append(d, r.Address...)
	}
	if r.Command&OW_READ > 0 {
		d = append(d, byte(r.ReadCount&0xff))
		d = append(d, byte(r.ReadCount>>8&0xff))
		d = append(d, byte(r.CorrelationId&0xff))
		d = append(d, byte(r.CorrelationId>>8&0xff))
	}
	if r.Command&OW_DELAY > 0 {
		d = append(d, byte(r.DelayMs&0xff))
		d = append(d, byte(r.DelayMs>>8&0xff))
		d = append(d, byte(r.DelayMs>>16&0xff))
		d = append(d, byte(r.DelayMs>>24&0xff))
	}
	if r.Command&OW_WRITE > 0 {
		d = append(d, r.Data...)
	}
	d = To7BitMulti(d)
	return d
}

// OneWireCrc8 calculates the 8 bit CRC of the data.
func OneWireCrc8(data []byte) byte {
	var crc byte
	for _, b := range data {
		for i := 8; i > 0; i-- {
			var mix uint8 = (crc ^ b) & 0x1
			crc >>= 1
			if mix > 0 {
				crc ^= 0x8C
			}
			b >>= 1
		}
	}
	return crc
}

// OneWireConfig configures a pin as a OneWire interface.
func (c *FirmataClient) OneWireConfig(csPin byte, owPowerMode byte) (err error) {
	csPinBytes := to7Bit(csPin)
	powerModeBytes := to7Bit(owPowerMode)
	c.owChan = make(chan []byte)

	err = c.sendSysEx(SysExOneWire, byte(OneWireConfig),
		csPinBytes[0], powerModeBytes[0])
	return
}

// OneWireSearch initiates a search on the OneWire bus.
func (c *FirmataClient) OneWireSearch(csPin byte, owSearchMode OneWireSubCommand) (addresses []OneWireAddress, err error) {
	err = c.sendSysEx(SysExOneWire, byte(owSearchMode), csPin)
	dataOut := <-c.owChan
	t := make(OneWireAddress, 0)
	for i, d := range dataOut {
		t = append(t, d)
		if i > 0 && i%8 == 7 {
			addresses = append(addresses, t)
			t = make(OneWireAddress, 0)
		}
	}
	return
}

// OneWireCommand initiates a command on the OneWire bus.
func (c *FirmataClient) OneWireCommand(csPin byte, request OneWireRequest) ([]byte, error) {
	var dataOut []byte
	var d []byte
	d = append(d, byte(request.Command))
	d = append(d, csPin)
	d = append(d, request.Encode()...)
	err := c.sendSysEx(SysExOneWire, d...)
	if err != nil {
		return nil, err
	}
	if request.Command&0x8 > 0 {
		dataOut = <-c.owChan
	}
	return dataOut, nil
}

// parseOWResponse handles a OneWire SysEx response packet.
func (c *FirmataClient) parseOWResponse(data7bit []byte) {
	data := From7BitMulti(data7bit)
	c.owChan <- data
}

// Ds18x20 is a Maxim DS1820 or DS18B20 device.
type Ds18x20 struct {
	// The client.
	Client *FirmataClient
	// The pin the bus is on.
	Pin byte
	// The address of the device on the bus.
	Address OneWireAddress
	// scratch is the raw register data.
	scratch []byte
	// Latest temperature reading.
	Temperature float32
	// The TH register.
	RegisterTh byte
	// The TL register.
	RegisterTl byte
	// The config register.
	ConfigRegister byte
}

// ConvertT initiates a temperature conversion.
func (d *Ds18x20) ConvertT(all bool) error {
	var req OneWireRequest
	req.Data = []byte{0x44}
	if all {
		req.Command = OW_RESET | OW_SELECT | OW_WRITE
		req.Address = d.Address
	} else {
		req.Command = OW_RESET | OW_SKIP | OW_WRITE
	}

	_, err := d.Client.OneWireCommand(d.Pin, req)
	return err
}

// ReadScratchPad reads the device scratchpad.
func (d *Ds18x20) ReadScratchPad() error {
	req := OneWireRequest{
		Command:       OW_RESET | OW_SELECT | OW_WRITE | OW_READ,
		Address:       d.Address,
		ReadCount:     9,
		CorrelationId: 0x1234,
		Data:          []byte{0xbe},
	}
	scratch, err := d.Client.OneWireCommand(d.Pin, req)
	if err != nil {
		return err
	}
	d.scratch = scratch[2:]
	crc := d.scratch[len(d.scratch)-1]
	c := OneWireCrc8(d.scratch[:len(d.scratch)-1])
	if c != crc {
		return fmt.Errorf("crc mismatch! Received 0x%x, calculated 0x%x! [0x%x]", crc, c, d.scratch)
	}
	d.parseTemperature()
	d.ConfigRegister = d.scratch[4]
	d.RegisterTh = d.scratch[2]
	d.RegisterTl = d.scratch[3]
	return nil
}

// Resolution sets the thermometer resolution, in bits.
func (d *Ds18x20) Resolution(r byte) error {
	if r < 9 || r > 12 {
		return fmt.Errorf("resolution must be between 9 and 12!")
	}
	req := OneWireRequest{
		Command:       OW_RESET | OW_SELECT | OW_WRITE,
		Address:       d.Address,
		CorrelationId: 0x1234,
		Data:          []byte{0x4e, 0x0, 0x0},
	}
	r = (r - 9) << 6
	req.Data = append(req.Data, r)
	_, err := d.Client.OneWireCommand(d.Pin, req)
	return err
}

// parseTemperature parses the raw temperature data.
func (d *Ds18x20) parseTemperature() {
	raw := uint16(d.scratch[0]) | uint16(d.scratch[1])<<8
	if d.Address[0] == 0x10 {
		raw = raw << 3
		d.Temperature = float32(raw&0xFFF0 + 12 - uint16(d.scratch[6]))
	} else {
		// Zero out bits that are undefined at lower resolutions.
		switch d.GetResolution() {
		case 9:
			raw &= 0xfff8
		case 10:
			raw &= 0xfffc
		case 11:
			raw &= 0xfffe
		}
		d.Temperature = float32(raw)
	}
	d.Temperature /= 16
}

// GetResolution gets the device temperature resolution in bits.
func (d *Ds18x20) GetResolution() byte {
	res := (d.ConfigRegister >> 5) & 0x3
	return res + 9
}
