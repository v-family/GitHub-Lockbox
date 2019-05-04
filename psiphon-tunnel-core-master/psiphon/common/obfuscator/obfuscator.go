/*
 * Copyright (c) 2015, Psiphon Inc.
 * All rights reserved.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package obfuscator

import (
	"bytes"
	"crypto/rc4"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"io"

	"github.com/Psiphon-Labs/psiphon-tunnel-core/psiphon/common"
	"github.com/Psiphon-Labs/psiphon-tunnel-core/psiphon/common/prng"
)

const (
	OBFUSCATE_SEED_LENGTH         = 16
	OBFUSCATE_KEY_LENGTH          = 16
	OBFUSCATE_HASH_ITERATIONS     = 6000
	OBFUSCATE_MAX_PADDING         = 8192
	OBFUSCATE_MAGIC_VALUE         = 0x0BF5CA7E
	OBFUSCATE_CLIENT_TO_SERVER_IV = "client_to_server"
	OBFUSCATE_SERVER_TO_CLIENT_IV = "server_to_client"
)

// Obfuscator implements the seed message, key derivation, and
// stream ciphers for:
// https://github.com/brl/obfuscated-openssh/blob/master/README.obfuscation
//
// Limitation: the RC4 cipher is vulnerable to ciphertext malleability and
// the "magic" value provides only weak authentication due to its small
// size. Increasing the size of the magic field will break compatibility
// with legacy clients. New protocols and schemes should not use this
// obfuscator.
type Obfuscator struct {
	seedMessage          []byte
	paddingLength        int
	clientToServerCipher *rc4.Cipher
	serverToClientCipher *rc4.Cipher
	paddingPRNGSeed      *prng.Seed
	paddingPRNG          *prng.PRNG
}

type ObfuscatorConfig struct {
	Keyword         string
	PaddingPRNGSeed *prng.Seed
	MinPadding      *int
	MaxPadding      *int
}

// NewClientObfuscator creates a new Obfuscator, staging a seed message to be
// sent to the server (by the caller) and initializing stream ciphers to
// obfuscate data.
//
// ObfuscatorConfig.PaddingPRNGSeed allows for optional replay of the
// obfuscator padding and must not be nil.
func NewClientObfuscator(
	config *ObfuscatorConfig) (obfuscator *Obfuscator, err error) {

	if config.PaddingPRNGSeed == nil {
		return nil, common.ContextError(
			errors.New("missing padding seed"))
	}

	paddingPRNG := prng.NewPRNGWithSeed(config.PaddingPRNGSeed)

	obfuscatorSeed, err := common.MakeSecureRandomBytes(OBFUSCATE_SEED_LENGTH)
	if err != nil {
		return nil, common.ContextError(err)
	}

	clientToServerCipher, serverToClientCipher, err := initObfuscatorCiphers(obfuscatorSeed, config)
	if err != nil {
		return nil, common.ContextError(err)
	}

	// The first prng.SEED_LENGTH bytes of the initial obfuscator message
	// padding field is used by the server as a seed for its obfuscator
	// padding and other protocol attributes (directly and via
	// GetDerivedPRNG). This allows for optional downstream replay of these
	// protocol attributes. Accordingly, the minimum padding is set to at
	// least prng.SEED_LENGTH.

	minPadding := prng.SEED_LENGTH
	if config.MinPadding != nil &&
		*config.MinPadding >= prng.SEED_LENGTH &&
		*config.MinPadding <= OBFUSCATE_MAX_PADDING {
		minPadding = *config.MinPadding
	}

	maxPadding := OBFUSCATE_MAX_PADDING
	if config.MaxPadding != nil &&
		*config.MaxPadding >= prng.SEED_LENGTH &&
		*config.MaxPadding <= OBFUSCATE_MAX_PADDING &&
		*config.MaxPadding >= minPadding {
		maxPadding = *config.MaxPadding
	}

	seedMessage, paddingLength, err := makeSeedMessage(
		paddingPRNG, minPadding, maxPadding, obfuscatorSeed, clientToServerCipher)
	if err != nil {
		return nil, common.ContextError(err)
	}

	return &Obfuscator{
		seedMessage:          seedMessage,
		paddingLength:        paddingLength,
		clientToServerCipher: clientToServerCipher,
		serverToClientCipher: serverToClientCipher,
		paddingPRNGSeed:      config.PaddingPRNGSeed,
		paddingPRNG:          paddingPRNG}, nil
}

// NewServerObfuscator creates a new Obfuscator, reading a seed message directly
// from the clientReader and initializing stream ciphers to obfuscate data.
//
// ObfuscatorConfig.PaddingPRNGSeed is not used, as the server obtains a PRNG
// seed from the client's initial obfuscator message; this scheme allows for
// optional replay of the downstream obfuscator padding.
func NewServerObfuscator(
	clientReader io.Reader, config *ObfuscatorConfig) (obfuscator *Obfuscator, err error) {

	clientToServerCipher, serverToClientCipher, paddingPRNGSeed, err := readSeedMessage(
		clientReader, config)
	if err != nil {
		return nil, common.ContextError(err)
	}

	return &Obfuscator{
		paddingLength:        -1,
		clientToServerCipher: clientToServerCipher,
		serverToClientCipher: serverToClientCipher,
		paddingPRNGSeed:      paddingPRNGSeed,
		paddingPRNG:          prng.NewPRNGWithSeed(paddingPRNGSeed),
	}, nil
}

// GetDerivedPRNG creates a new PRNG with a seed derived from the obfuscator
// padding seed and distinguished by the salt, which should be a unique
// identifier for each usage context.
//
// For NewServerObfuscator, the obfuscator padding seed is obtained from the
// client, so derived PRNGs may be used to replay sequences post-initial
// obfuscator message.
func (obfuscator *Obfuscator) GetDerivedPRNG(salt string) (*prng.PRNG, error) {
	return prng.NewPRNGWithSaltedSeed(obfuscator.paddingPRNGSeed, salt)
}

// GetPaddingLength returns the client seed message padding length. Only valid
// for NewClientObfuscator.
func (obfuscator *Obfuscator) GetPaddingLength() int {
	return obfuscator.paddingLength
}

// SendSeedMessage returns the seed message created in NewObfuscatorClient,
// removing the reference so that it may be garbage collected.
func (obfuscator *Obfuscator) SendSeedMessage() []byte {
	seedMessage := obfuscator.seedMessage
	obfuscator.seedMessage = nil
	return seedMessage
}

// ObfuscateClientToServer applies the client RC4 stream to the bytes in buffer.
func (obfuscator *Obfuscator) ObfuscateClientToServer(buffer []byte) {
	obfuscator.clientToServerCipher.XORKeyStream(buffer, buffer)
}

// ObfuscateServerToClient applies the server RC4 stream to the bytes in buffer.
func (obfuscator *Obfuscator) ObfuscateServerToClient(buffer []byte) {
	obfuscator.serverToClientCipher.XORKeyStream(buffer, buffer)
}

func initObfuscatorCiphers(
	obfuscatorSeed []byte, config *ObfuscatorConfig) (*rc4.Cipher, *rc4.Cipher, error) {

	clientToServerKey, err := deriveKey(obfuscatorSeed, []byte(config.Keyword), []byte(OBFUSCATE_CLIENT_TO_SERVER_IV))
	if err != nil {
		return nil, nil, common.ContextError(err)
	}

	serverToClientKey, err := deriveKey(obfuscatorSeed, []byte(config.Keyword), []byte(OBFUSCATE_SERVER_TO_CLIENT_IV))
	if err != nil {
		return nil, nil, common.ContextError(err)
	}

	clientToServerCipher, err := rc4.NewCipher(clientToServerKey)
	if err != nil {
		return nil, nil, common.ContextError(err)
	}

	serverToClientCipher, err := rc4.NewCipher(serverToClientKey)
	if err != nil {
		return nil, nil, common.ContextError(err)
	}

	return clientToServerCipher, serverToClientCipher, nil
}

func deriveKey(obfuscatorSeed, keyword, iv []byte) ([]byte, error) {
	h := sha1.New()
	h.Write(obfuscatorSeed)
	h.Write(keyword)
	h.Write(iv)
	digest := h.Sum(nil)
	for i := 0; i < OBFUSCATE_HASH_ITERATIONS; i++ {
		h.Reset()
		h.Write(digest)
		digest = h.Sum(nil)
	}
	if len(digest) < OBFUSCATE_KEY_LENGTH {
		return nil, common.ContextError(errors.New("insufficient bytes for obfuscation key"))
	}
	return digest[0:OBFUSCATE_KEY_LENGTH], nil
}

func makeSeedMessage(
	paddingPRNG *prng.PRNG,
	minPadding, maxPadding int,
	obfuscatorSeed []byte,
	clientToServerCipher *rc4.Cipher) ([]byte, int, error) {

	padding := paddingPRNG.Padding(minPadding, maxPadding)
	buffer := new(bytes.Buffer)
	err := binary.Write(buffer, binary.BigEndian, obfuscatorSeed)
	if err != nil {
		return nil, 0, common.ContextError(err)
	}
	err = binary.Write(buffer, binary.BigEndian, uint32(OBFUSCATE_MAGIC_VALUE))
	if err != nil {
		return nil, 0, common.ContextError(err)
	}
	err = binary.Write(buffer, binary.BigEndian, uint32(len(padding)))
	if err != nil {
		return nil, 0, common.ContextError(err)
	}
	err = binary.Write(buffer, binary.BigEndian, padding)
	if err != nil {
		return nil, 0, common.ContextError(err)
	}
	seedMessage := buffer.Bytes()
	clientToServerCipher.XORKeyStream(seedMessage[len(obfuscatorSeed):], seedMessage[len(obfuscatorSeed):])
	return seedMessage, len(padding), nil
}

func readSeedMessage(
	clientReader io.Reader, config *ObfuscatorConfig) (*rc4.Cipher, *rc4.Cipher, *prng.Seed, error) {

	seed := make([]byte, OBFUSCATE_SEED_LENGTH)
	_, err := io.ReadFull(clientReader, seed)
	if err != nil {
		return nil, nil, nil, common.ContextError(err)
	}

	clientToServerCipher, serverToClientCipher, err := initObfuscatorCiphers(seed, config)
	if err != nil {
		return nil, nil, nil, common.ContextError(err)
	}

	fixedLengthFields := make([]byte, 8) // 4 bytes each for magic value and padding length
	_, err = io.ReadFull(clientReader, fixedLengthFields)
	if err != nil {
		return nil, nil, nil, common.ContextError(err)
	}

	clientToServerCipher.XORKeyStream(fixedLengthFields, fixedLengthFields)

	buffer := bytes.NewReader(fixedLengthFields)

	// The magic value must be validated before acting on paddingLength as
	// paddingLength validation is vulnerable to a chosen ciphertext probing
	// attack: only a fixed number of any possible byte value for each
	// paddingLength is valid.

	var magicValue, paddingLength int32
	err = binary.Read(buffer, binary.BigEndian, &magicValue)
	if err != nil {
		return nil, nil, nil, common.ContextError(err)
	}
	err = binary.Read(buffer, binary.BigEndian, &paddingLength)
	if err != nil {
		return nil, nil, nil, common.ContextError(err)
	}

	if magicValue != OBFUSCATE_MAGIC_VALUE {
		return nil, nil, nil, common.ContextError(errors.New("invalid magic value"))
	}

	if paddingLength < 0 || paddingLength > OBFUSCATE_MAX_PADDING {
		return nil, nil, nil, common.ContextError(errors.New("invalid padding length"))
	}

	padding := make([]byte, paddingLength)
	_, err = io.ReadFull(clientReader, padding)
	if err != nil {
		return nil, nil, nil, common.ContextError(err)
	}

	clientToServerCipher.XORKeyStream(padding, padding)

	// Use the first prng.SEED_LENGTH bytes of padding as a PRNG seed for
	// subsequent operations. This allows the client to direct server-side
	// replay of certain protocol attributes.
	//
	// Since legacy clients may send < prng.SEED_LENGTH bytes of padding,
	// generate a new seed in that case.

	var paddingPRNGSeed *prng.Seed

	if len(padding) >= prng.SEED_LENGTH {
		paddingPRNGSeed = new(prng.Seed)
		copy(paddingPRNGSeed[:], padding[0:prng.SEED_LENGTH])
	} else {
		paddingPRNGSeed, err = prng.NewSeed()
		if err != nil {
			return nil, nil, nil, common.ContextError(err)
		}
	}

	return clientToServerCipher, serverToClientCipher, paddingPRNGSeed, nil
}
