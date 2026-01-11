module github.com/unblink/unblink/relay

go 1.24.0

require (
	github.com/AlexxIT/go2rtc v1.9.13
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/pion/webrtc/v4 v4.2.1
	github.com/tursodatabase/turso-go v0.2.2
	github.com/unblink/unblink/node v0.0.0
	golang.org/x/crypto v0.46.0
)

replace github.com/unblink/unblink/node => ../node

require (
	github.com/ebitengine/purego v0.10.0-alpha.2 // indirect
	github.com/fxamacker/cbor/v2 v2.5.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/pion/datachannel v1.5.10 // indirect
	github.com/pion/dtls/v3 v3.0.9 // indirect
	github.com/pion/ice/v4 v4.1.0 // indirect
	github.com/pion/interceptor v0.1.42 // indirect
	github.com/pion/logging v0.2.4 // indirect
	github.com/pion/mdns/v2 v2.1.0 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtcp v1.2.16 // indirect
	github.com/pion/rtp v1.8.27 // indirect
	github.com/pion/sctp v1.9.0 // indirect
	github.com/pion/sdp/v3 v3.0.17 // indirect
	github.com/pion/srtp/v3 v3.0.9 // indirect
	github.com/pion/stun/v3 v3.0.2 // indirect
	github.com/pion/transport/v3 v3.1.1 // indirect
	github.com/pion/turn/v4 v4.1.3 // indirect
	github.com/sigurn/crc16 v0.0.0-20240131213347-83fcde1e29d1 // indirect
	github.com/sigurn/crc8 v0.0.0-20220107193325-2243fe600f9f // indirect
	github.com/tidwall/jsonc v0.3.2 // indirect
	github.com/wlynxg/anet v0.0.5 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
)
