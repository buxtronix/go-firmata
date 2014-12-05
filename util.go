// Copyright 2014 Krishna Raman
// 
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// 
//     http://www.apache.org/licenses/LICENSE-2.0
// 
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package firmata

func from7Bit(b0 byte, b1 byte) byte {
	return (b0 & 0x7F) | ((b1 & 0x7F) << 7)
}

func to7Bit(i byte) []byte {
	return []byte{i & 0x7f, (i >> 7) & 0x7f}
}

func intto7Bit(i int) []byte {
	return []byte{byte(i & 0x7f), byte((i >> 7) & 0x7f), byte((i >> 14) & 0x7f)}
}

func multibyteString(data []byte) (str string) {

	if len(data)%2 != 0 {
		data = append(data, 0)
	}

	for i := 0; i < len(data); i = i + 2 {
		str = str + string(from7Bit(data[i], data[i+1]))
	}

	return
}

// from7BitMulti converts 7 bit encoded data to 8 bit.
func From7BitMulti(data []byte) []byte {
  var i uint
  res := make([]byte, 0)
  var shift uint = 0
  for i = 0 ; int(i) < len(data)-3 ; i++ {
    if i > 0 && i % 7 == 0 {
      i++
    }
    j := i + 2
    d := data[j] >> shift
    d |= data[j+1] << (7-shift)
    shift++
    if shift > 6 {
      shift = 0
    }
    res = append(res, d)
  }
  return res
}

// to7BitMulti converts 8 bit encoded data to 7 bit.
func To7BitMulti(data []byte) []byte {
  var i uint
  var prev byte
  var res []byte
  var shift uint = 0
  for i = 0 ; int(i) < len(data) ; i++ {
    if shift == 0 {
      res = append(res, data[i] & 0x7f)
      shift++
      prev = data[i] >> 7
    } else {
      res = append(res, ((data[i] << shift) & 0x7f) | prev)
      if shift == 6 {
        res = append(res, data[i] >> 1)
        shift = 0
      } else {
        shift++
        prev = data[i] >> (8 - shift)
      }
    }
  }
  if shift > 0 {
    res = append(res, prev)
  }
  return res
}
