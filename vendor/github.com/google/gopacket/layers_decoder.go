// Copyright 2019 The GoPacket Authors. All rights reserved.

package gopacket

// Created by gen.go, don't edit manually
// Generated at 2019-06-18 11:37:31.308731293 +0600 +06 m=+0.000842599

// LayersDecoder returns DecodingLayerFunc for specified
// DecodingLayerContainer, LayerType value to start decoding with and
// some DecodeFeedback.
func LayersDecoder(dl DecodingLayerContainer, first LayerType, df DecodeFeedback) DecodingLayerFunc {
	firstDec, ok := dl.Decoder(first)
	if !ok {
		return func([]byte, *[]LayerType) (LayerType, error) {
			return first, nil
		}
	}
	if dlc, ok := dl.(DecodingLayerSparse); ok {
		return func(data []byte, decoded *[]LayerType) (LayerType, error) {
			*decoded = (*decoded)[:0] // Truncated decoded layers.
			typ := first
			decoder := firstDec
			for {
				if err := decoder.DecodeFromBytes(data, df); err != nil {
					return LayerTypeZero, err
				}
				*decoded = append(*decoded, typ)
				typ = decoder.NextLayerType()
				if data = decoder.LayerPayload(); len(data) == 0 {
					break
				}
				if decoder, ok = dlc.Decoder(typ); !ok {
					return typ, nil
				}
			}
			return LayerTypeZero, nil
		}
	}
	if dlc, ok := dl.(DecodingLayerArray); ok {
		return func(data []byte, decoded *[]LayerType) (LayerType, error) {
			*decoded = (*decoded)[:0] // Truncated decoded layers.
			typ := first
			decoder := firstDec
			for {
				if err := decoder.DecodeFromBytes(data, df); err != nil {
					return LayerTypeZero, err
				}
				*decoded = append(*decoded, typ)
				typ = decoder.NextLayerType()
				if data = decoder.LayerPayload(); len(data) == 0 {
					break
				}
				if decoder, ok = dlc.Decoder(typ); !ok {
					return typ, nil
				}
			}
			return LayerTypeZero, nil
		}
	}
	if dlc, ok := dl.(DecodingLayerMap); ok {
		return func(data []byte, decoded *[]LayerType) (LayerType, error) {
			*decoded = (*decoded)[:0] // Truncated decoded layers.
			typ := first
			decoder := firstDec
			for {
				if err := decoder.DecodeFromBytes(data, df); err != nil {
					return LayerTypeZero, err
				}
				*decoded = append(*decoded, typ)
				typ = decoder.NextLayerType()
				if data = decoder.LayerPayload(); len(data) == 0 {
					break
				}
				if decoder, ok = dlc.Decoder(typ); !ok {
					return typ, nil
				}
			}
			return LayerTypeZero, nil
		}
	}
	dlc := dl
	return func(data []byte, decoded *[]LayerType) (LayerType, error) {
		*decoded = (*decoded)[:0] // Truncated decoded layers.
		typ := first
		decoder := firstDec
		for {
			if err := decoder.DecodeFromBytes(data, df); err != nil {
				return LayerTypeZero, err
			}
			*decoded = append(*decoded, typ)
			typ = decoder.NextLayerType()
			if data = decoder.LayerPayload(); len(data) == 0 {
				break
			}
			if decoder, ok = dlc.Decoder(typ); !ok {
				return typ, nil
			}
		}
		return LayerTypeZero, nil
	}
}
