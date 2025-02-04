package sequencer_test

import (
	"context"
	"testing"
	"time"

	"github.com/0xPolygon/cdk-data-availability/config"
	"github.com/0xPolygon/cdk-data-availability/config/types"
	"github.com/0xPolygon/cdk-data-availability/etherman/smartcontracts/etrog/polygonvalidium"
	"github.com/0xPolygon/cdk-data-availability/mocks"
	"github.com/0xPolygon/cdk-data-availability/sequencer"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTracker(t *testing.T) {
	var (
		initialAddress = common.BytesToAddress([]byte("initial"))
		initialURL     = "127.0.0.1:8585"
		updatedAddress = common.BytesToAddress([]byte("updated"))
		updatedURL     = "127.0.0.1:9585"
	)

	t.Run("with enabled tracker", func(t *testing.T) {
		var (
			addressesChan chan *polygonvalidium.PolygonvalidiumSetTrustedSequencer
			urlsChan      chan *polygonvalidium.PolygonvalidiumSetTrustedSequencerURL
		)

		ctx := context.Background()

		etherman := mocks.NewEtherman(t)

		etherman.On("TrustedSequencer", mock.Anything).Return(initialAddress, nil)
		etherman.On("TrustedSequencerURL", mock.Anything).Return(initialURL, nil)

		addressesSubscription := mocks.NewSubscription(t)

		addressesSubscription.On("Err").Return(make(<-chan error))
		addressesSubscription.On("Unsubscribe").Return()

		etherman.On("WatchSetTrustedSequencer", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				var ok bool
				addressesChan, ok = args[1].(chan *polygonvalidium.PolygonvalidiumSetTrustedSequencer)
				require.True(t, ok)
			}).
			Return(addressesSubscription, nil)

		urlsSubscription := mocks.NewSubscription(t)

		urlsSubscription.On("Err").Return(make(<-chan error))
		urlsSubscription.On("Unsubscribe").Return()

		etherman.On("WatchSetTrustedSequencerURL", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				var ok bool
				urlsChan, ok = args[1].(chan *polygonvalidium.PolygonvalidiumSetTrustedSequencerURL)
				require.True(t, ok)
			}).
			Return(urlsSubscription, nil)

		tracker := sequencer.NewTracker(config.L1Config{
			Timeout:        types.NewDuration(time.Second * 10),
			RetryPeriod:    types.NewDuration(time.Millisecond),
			TrackSequencer: true,
		}, etherman)

		require.Equal(t, common.Address{}, tracker.GetAddr())
		require.Empty(t, tracker.GetUrl())

		tracker.Start(ctx)

		require.Equal(t, initialAddress, tracker.GetAddr())
		require.Equal(t, initialURL, tracker.GetUrl())

		eventually(t, 10, func() bool {
			return addressesChan != nil && urlsChan != nil
		})

		addressesChan <- &polygonvalidium.PolygonvalidiumSetTrustedSequencer{
			NewTrustedSequencer: updatedAddress,
		}

		urlsChan <- &polygonvalidium.PolygonvalidiumSetTrustedSequencerURL{
			NewTrustedSequencerURL: updatedURL,
		}

		tracker.Stop()

		// Wait for values to be updated
		eventually(t, 10, func() bool {
			return tracker.GetAddr() == updatedAddress && tracker.GetUrl() == updatedURL
		})

		urlsSubscription.AssertExpectations(t)
		addressesSubscription.AssertExpectations(t)
		etherman.AssertExpectations(t)
	})

	t.Run("with disabled tracker", func(t *testing.T) {
		ctx := context.Background()

		etherman := mocks.NewEtherman(t)

		etherman.On("TrustedSequencer", mock.Anything).Return(initialAddress, nil)
		etherman.On("TrustedSequencerURL", mock.Anything).Return(initialURL, nil)

		tracker := sequencer.NewTracker(config.L1Config{
			Timeout:     types.NewDuration(time.Second * 10),
			RetryPeriod: types.NewDuration(time.Millisecond),
		}, etherman)

		require.Equal(t, common.Address{}, tracker.GetAddr())
		require.Empty(t, tracker.GetUrl())

		tracker.Start(ctx)

		require.Equal(t, initialAddress, tracker.GetAddr())
		require.Equal(t, initialURL, tracker.GetUrl())

		tracker.Stop()

		etherman.AssertExpectations(t)
	})
}

func eventually(t *testing.T, num int, f func() bool) {
	t.Helper()

	for i := 0; i < num; i++ {
		if f() {
			return
		}

		time.Sleep(time.Second)
	}

	t.Failed()
}
