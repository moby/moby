Changes by Version
==================


1.2.0 (2020-07-01)
-------------------

* Restore the ability to reset the current span in context to nil (#231) -- Yuri Shkuro
* Use error.object per OpenTracing Semantic Conventions (#179) -- Rahman Syed
* Convert nil pointer log field value to string "nil" (#230) -- Cyril Tovena
* Add Go module support (#215) -- Zaba505
* Make SetTag helper types in ext public (#229) -- Blake Edwards
* Add log/fields helpers for keys from specification (#226) -- Dmitry Monakhov
* Improve noop impementation (#223) -- chanxuehong
* Add an extension to Tracer interface for custom go context creation (#220) -- Krzesimir Nowak
* Fix typo in comments (#222) -- meteorlxy
* Improve documentation for log.Object() to emphasize the requirement to pass immutable arguments (#219) -- 疯狂的小企鹅
* [mock] Return ErrInvalidSpanContext if span context is not MockSpanContext (#216) -- Milad Irannejad


1.1.0 (2019-03-23)
-------------------

Notable changes:
- The library is now released under Apache 2.0 license
- Use Set() instead of Add() in HTTPHeadersCarrier is functionally a breaking change (fixes issue [#159](https://github.com/opentracing/opentracing-go/issues/159))
- 'golang.org/x/net/context' is replaced with 'context' from the standard library

List of all changes:

- Export StartSpanFromContextWithTracer (#214) <Aaron Delaney>
- Add IsGlobalTracerRegistered() to indicate if a tracer has been registered (#201) <Mike Goldsmith>
- Use Set() instead of Add() in HTTPHeadersCarrier (#191) <jeremyxu2010>
- Update license to Apache 2.0 (#181) <Andrea Kao>
- Replace 'golang.org/x/net/context' with 'context' (#176) <Tony Ghita>
- Port of Python opentracing/harness/api_check.py to Go (#146) <chris erway>
- Fix race condition in MockSpan.Context() (#170) <Brad>
- Add PeerHostIPv4.SetString() (#155)  <NeoCN>
- Add a Noop log field type to log to allow for optional fields (#150)  <Matt Ho>


1.0.2 (2017-04-26)
-------------------

- Add more semantic tags (#139) <Rustam Zagirov>


1.0.1 (2017-02-06)
-------------------

- Correct spelling in comments <Ben Sigelman>
- Address race in nextMockID() (#123) <bill fumerola>
- log: avoid panic marshaling nil error (#131) <Anthony Voutas>
- Deprecate InitGlobalTracer in favor of SetGlobalTracer (#128) <Yuri Shkuro>
- Drop Go 1.5 that fails in Travis (#129) <Yuri Shkuro>
- Add convenience methods Key() and Value() to log.Field <Ben Sigelman>
- Add convenience methods to log.Field (2 years, 6 months ago) <Radu Berinde>

1.0.0 (2016-09-26)
-------------------

- This release implements OpenTracing Specification 1.0 (https://opentracing.io/spec)

