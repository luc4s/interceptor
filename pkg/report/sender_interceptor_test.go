// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package report

import (
	"testing"
	"time"

	"github.com/luc4s/interceptor/internal/ntp"
	"github.com/luc4s/interceptor/internal/test"
	"github.com/pion/interceptor"
	"github.com/pion/logging"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/assert"
)

func TestSenderInterceptor(t *testing.T) {
	t.Run("before any packet", func(t *testing.T) {
		mt := &test.MockTime{}
		f, err := NewSenderInterceptor(
			SenderInterval(time.Millisecond*50),
			SenderLog(logging.NewDefaultLoggerFactory().NewLogger("test")),
			SenderNow(mt.Now),
		)
		assert.NoError(t, err)

		i, err := f.NewInterceptor("")
		assert.NoError(t, err)

		stream := test.NewMockStream(&interceptor.StreamInfo{
			SSRC:      123456,
			ClockRate: 90000,
		}, i)
		defer func() {
			assert.NoError(t, stream.Close())
		}()

		mt.SetNow(time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC))
		pkts := <-stream.WrittenRTCP()
		assert.Equal(t, len(pkts), 1)
		sr, ok := pkts[0].(*rtcp.SenderReport)
		assert.True(t, ok)
		assert.Equal(t, &rtcp.SenderReport{
			SSRC:        123456,
			NTPTime:     ntp.ToNTP(mt.Now()),
			RTPTime:     2269117121,
			PacketCount: 0,
			OctetCount:  0,
		}, sr)
	})

	t.Run("after RTP packets", func(t *testing.T) {
		mt := &test.MockTime{}
		f, err := NewSenderInterceptor(
			SenderInterval(time.Millisecond*50),
			SenderLog(logging.NewDefaultLoggerFactory().NewLogger("test")),
			SenderNow(mt.Now),
		)
		assert.NoError(t, err)

		i, err := f.NewInterceptor("")
		assert.NoError(t, err)

		stream := test.NewMockStream(&interceptor.StreamInfo{
			SSRC:      123456,
			ClockRate: 90000,
		}, i)
		defer func() {
			assert.NoError(t, stream.Close())
		}()

		for i := 0; i < 10; i++ {
			assert.NoError(t, stream.WriteRTP(&rtp.Packet{
				Header:  rtp.Header{SequenceNumber: uint16(i)},
				Payload: []byte("\x00\x00"),
			}))
		}

		mt.SetNow(time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC))
		pkts := <-stream.WrittenRTCP()
		assert.Equal(t, len(pkts), 1)
		sr, ok := pkts[0].(*rtcp.SenderReport)
		assert.True(t, ok)
		assert.Equal(t, &rtcp.SenderReport{
			SSRC:        123456,
			NTPTime:     ntp.ToNTP(mt.Now()),
			RTPTime:     2269117121,
			PacketCount: 10,
			OctetCount:  20,
		}, sr)
	})

	t.Run("inject ticker", func(t *testing.T) {
		mNow := &test.MockTime{}
		mTick := &test.MockTicker{
			C: make(chan time.Time),
		}
		advanceTicker := func() {
			mNow.SetNow(mNow.Now().Add(50 * time.Millisecond))
			mTick.Tick(mNow.Now())
		}
		loopStarted := make(chan struct{})
		f, err := NewSenderInterceptor(
			SenderInterval(time.Millisecond*50),
			SenderLog(logging.NewDefaultLoggerFactory().NewLogger("test")),
			SenderNow(mNow.Now),
			SenderTicker(func(d time.Duration) Ticker { return mTick }),
			enableStartTracking(loopStarted),
		)
		assert.NoError(t, err)

		i, err := f.NewInterceptor("")
		assert.NoError(t, err)

		stream := test.NewMockStream(&interceptor.StreamInfo{
			SSRC:      123456,
			ClockRate: 90000,
		}, i)
		defer func() {
			assert.NoError(t, stream.Close())
		}()

		<-loopStarted
		for i := 0; i < 5; i++ {
			advanceTicker()
			pkts := <-stream.WrittenRTCP()
			assert.Equal(t, len(pkts), 1)
		}
	})
}
