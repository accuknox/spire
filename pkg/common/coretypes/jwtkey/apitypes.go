package jwtkey

import (
	plugintypes "github.com/accuknox/spire-plugin-sdk/proto/spire/plugin/types"
	apitypes "github.com/spiffe/spire-api-sdk/proto/spire/api/types"
)

func ToAPIProto(jwtKey JWTKey) (*apitypes.JWTKey, error) {
	id, publicKey, expiresAt, err := toProtoFields(jwtKey)
	if err != nil {
		return nil, err
	}

	return &apitypes.JWTKey{
		KeyId:     id,
		PublicKey: publicKey,
		ExpiresAt: expiresAt,
	}, nil
}

func ToAPIFromPluginProto(pb *plugintypes.JWTKey) (*apitypes.JWTKey, error) {
	if pb == nil {
		return nil, nil
	}

	jwtKey, err := fromProtoFields(pb.KeyId, pb.PublicKey, pb.ExpiresAt)
	if err != nil {
		return nil, err
	}

	return ToAPIProto(jwtKey)
}
