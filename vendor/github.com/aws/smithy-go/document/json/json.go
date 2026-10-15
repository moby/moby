package json

// NewEncoder returns an Encoder for serializing Smithy documents for JSON based protocols.
func NewEncoder(optFns ...func(options *EncoderOptions)) *Encoder {
	o := EncoderOptions{}

	for _, fn := range optFns {
		fn(&o)
	}

	return &Encoder{
		options: o,
	}
}

// NewDecoder returns a Decoder for deserializing Smithy documents for JSON based protocols.
func NewDecoder(optFns ...func(*DecoderOptions)) *Decoder {
	o := DecoderOptions{}

	for _, fn := range optFns {
		fn(&o)
	}

	return &Decoder{
		options: o,
	}
}
