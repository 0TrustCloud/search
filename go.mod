module github.com/0TrustCloud/search

go 1.25.10

require (
	github.com/0TrustCloud/guikit v1.1.3
	github.com/0TrustCloud/orchid_log v0.0.0
	github.com/0TrustCloud/orchid_sync v0.0.0
	github.com/0TrustCloud/product_otrust v0.0.0
	github.com/0TrustCloud/product_security v0.0.0
	github.com/0TrustCloud/ultimate_db v1.3.5
)

require (
	github.com/0TrustCloud/auth_provider v1.0.1 // indirect
	github.com/0TrustCloud/logger v1.0.3-0.20260531010651-0732aad9e52f // indirect
	github.com/0TrustCloud/samln v0.0.0 // indirect
	github.com/0TrustCloud/secure_data_format v1.0.0 // indirect
	github.com/0TrustCloud/secure_network v1.1.4 // indirect
	github.com/0TrustCloud/secure_policy v1.0.6-0.20260531002558-a3c918113eef // indirect
	github.com/boombuler/barcode v1.0.1-0.20190219062509-6c824513bacc // indirect
	github.com/flynn/noise v1.1.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/go-webauthn/webauthn v0.17.4 // indirect
	github.com/go-webauthn/x v0.2.6 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pquerna/otp v1.5.0 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.59.1 // indirect
	github.com/tinylib/msgp v1.6.4 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
)

replace (
	github.com/0TrustCloud/auth_provider => ../../modules/auth_provider
	github.com/0TrustCloud/guikit => ../../modules/guikit
	github.com/0TrustCloud/logger => ../../modules/logger
	github.com/0TrustCloud/orchid_log => ../../modules/orchid_log
	github.com/0TrustCloud/orchid_sync => ../../modules/orchid_sync
	github.com/0TrustCloud/product_otrust => ../../modules/product_otrust
	github.com/0TrustCloud/product_security => ../../modules/product_security
	github.com/0TrustCloud/samln => ../../modules/samln
	github.com/0TrustCloud/secure_data_format => ../../modules/secure_data_format
	github.com/0TrustCloud/secure_network => ../../modules/secure_network
	github.com/0TrustCloud/secure_policy => ../../modules/secure_policy
	github.com/0TrustCloud/ultimate_db => ../../modules/ultimate_db
)
