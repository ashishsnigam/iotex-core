// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package genesis

import (
	"flag"
	"io/ioutil"
	"math/big"
	"sort"
	"time"

	"github.com/iotexproject/iotex-core/pkg/hash"

	"github.com/pkg/errors"
	"go.uber.org/config"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-election/committee"
)

var (
	// Default contains the default genesis config
	Default     Genesis
	genesisPath string
)

func init() {
	flag.StringVar(&genesisPath, "genesis-path", "", "Genesis path")
	initDefaultConfig()
}

func initDefaultConfig() {
	Default = Genesis{
		Blockchain: Blockchain{
			Timestamp:             1546329600,
			BlockGasLimit:         20000000,
			ActionGasLimit:        5000000,
			BlockInterval:         10 * time.Second,
			NumSubEpochs:          2,
			NumDelegates:          24,
			NumCandidateDelegates: 36,
			TimeBasedRotation:     false,
		},
		Account: Account{
			InitBalanceMap: make(map[string]string),
		},
		Poll: Poll{
			EnableBeaconChainVoting: false,
			CommitteeConfig: committee.Config{
				BeaconChainAPIs: []string{},
			},
		},
		Rewarding: Rewarding{
			InitAdminAddrStr:           identityset.Address(0).String(),
			InitBalanceStr:             unit.ConvertIotxToRau(1200000000).String(),
			BlockRewardStr:             unit.ConvertIotxToRau(36).String(),
			EpochRewardStr:             unit.ConvertIotxToRau(400000).String(),
			NumDelegatesForEpochReward: 100,
		},
	}
	for i := 0; i < identityset.Size(); i++ {
		addr := identityset.Address(i).String()
		value := unit.ConvertIotxToRau(100000000).String()
		Default.InitBalanceMap[addr] = value
		if uint64(i) < Default.NumDelegates {
			Default.Delegates = append(Default.Delegates, Delegate{
				OperatorAddrStr: addr,
				RewardAddrStr:   addr,
				VotesStr:        value,
			})
		}
	}
}

type (
	// Genesis is the root level of genesis config. Genesis config is the network-wide blockchain config. All the nodes
	// participating into the same network should use EXACTLY SAME genesis config.
	Genesis struct {
		Blockchain `yaml:"blockchain"`
		Account    `ymal:"account"`
		Poll       `yaml:"poll"`
		Rewarding  `yaml:"rewarding"`
		// Digest is the digest of genesis config file
		Digest hash.Hash256
	}
	// Blockchain contains blockchain level configs
	Blockchain struct {
		// Timestamp is the timestamp of the genesis block
		Timestamp int64
		// BlockGasLimit is the total gas limit could be consumed in a block
		BlockGasLimit uint64 `yaml:"blockGasLimit"`
		// ActionGasLimit is the per action gas limit cap
		ActionGasLimit uint64 `yaml:"actionGasLimit"`
		// BlockInterval is the interval between two blocks
		BlockInterval time.Duration `yaml:"blockInterval"`
		// NumSubEpochs is the number of sub epochs in one epoch of block production
		NumSubEpochs uint64 `yaml:"numSubEpochs"`
		// NumDelegates is the number of delegates that participate into one epoch of block production
		NumDelegates uint64 `yaml:"numDelegates"`
		// NumCandidateDelegates is the number of candidate delegates, who may be selected as a delegate via roll dpos
		NumCandidateDelegates uint64 `yaml:"numCandidateDelegates"`
		// TimeBasedRotation is the flag to enable rotating delegates' time slots on a block height
		TimeBasedRotation bool `yaml:"timeBasedRotation"`
	}
	// Account contains the configs for account protocol
	Account struct {
		// InitBalanceMap is the address and initial balance mapping before the first block.
		InitBalanceMap map[string]string `yaml:"initBalances"`
	}
	// Poll contains the configs for poll protocol
	Poll struct {
		// EnableBeaconChainVoting is a flag whether read voting from beacon chain
		EnableBeaconChainVoting bool `yaml:"enableBeaconChainVoting"`
		// InitBeaconChainHeight is the height in beacon chain where the init poll result stored
		InitBeaconChainHeight uint64 `yaml:"initBeaconChainHeight"`
		// CommitteeConfig is the config for committee
		CommitteeConfig committee.Config `yaml:"committeeConfig"`
		// Delegates is a list of delegates with votes
		Delegates []Delegate `yaml:"delegates"`
	}
	// Delegate defines a delegate with address and votes
	Delegate struct {
		// OperatorAddrStr is the address who will operate the node
		OperatorAddrStr string `yaml:"operatorAddr"`
		// RewardAddrStr is the address who will get the reward when operator produces blocks
		RewardAddrStr string `yaml:"rewardAddr"`
		// VotesStr is the score for the operator to rank and weight for rewardee to split epoch reward
		VotesStr string `yaml:"votes"`
	}
	// Rewarding contains the configs for rewarding protocol
	Rewarding struct {
		// InitAdminAddrStr is the address of the initial rewarding protocol admin in encoded string format
		InitAdminAddrStr string `yaml:"initAdminAddr"`
		// InitBalanceStr is the initial balance of the rewarding protocol in decimal string format
		InitBalanceStr string `yaml:"initBalance"`
		// BlockReward is the block reward amount in decimal string format
		BlockRewardStr string `yaml:"blockReward"`
		// EpochReward is the epoch reward amount in decimal string format
		EpochRewardStr string `yaml:"epochReward"`
		// NumDelegatesForEpochReward is the number of top candidates that will share a epoch reward
		NumDelegatesForEpochReward uint64 `yaml:"numDelegatesForEpochReward"`
	}
)

// New constructs a genesis config. It loads the default values, and could be overwritten by values defined in the yaml
// config files
func New() (Genesis, error) {
	opts := make([]config.YAMLOption, 0)
	opts = append(opts, config.Static(Default))
	genesisDigest := hash.ZeroHash256
	if genesisPath != "" {
		opts = append(opts, config.File(genesisPath))
		genesisCfgBytes, err := ioutil.ReadFile(genesisPath)
		if err != nil {
			return Genesis{}, err
		}
		genesisDigest = hash.Hash256b(genesisCfgBytes)
	}
	yaml, err := config.NewYAML(opts...)
	if err != nil {
		return Genesis{}, errors.Wrap(err, "error when constructing a genesis in yaml")
	}

	var genesis Genesis
	if err := yaml.Get(config.Root).Populate(&genesis); err != nil {
		return Genesis{}, errors.Wrap(err, "failed to unmarshal yaml genesis to struct")
	}
	genesis.Digest = genesisDigest
	return genesis, nil
}

// InitBalances returns the address that have initial balances and the corresponding amounts. The i-th amount is the
// i-th address' balance.
func (a *Account) InitBalances() ([]address.Address, []*big.Int) {
	// Make the list always be ordered
	addrStrs := make([]string, 0)
	for addrStr := range a.InitBalanceMap {
		addrStrs = append(addrStrs, addrStr)
	}
	sort.Strings(addrStrs)
	addrs := make([]address.Address, 0)
	amounts := make([]*big.Int, 0)
	for _, addrStr := range addrStrs {
		addr, err := address.FromString(addrStr)
		if err != nil {
			log.L().Panic("Error when decoding the account protocol init balance address from string.", zap.Error(err))
		}
		addrs = append(addrs, addr)
		amount, ok := big.NewInt(0).SetString(a.InitBalanceMap[addrStr], 10)
		if !ok {
			log.S().Panicf("Error when casting init balance string %s into big int", a.InitBalanceMap[addrStr])
		}
		amounts = append(amounts, amount)
	}
	return addrs, amounts
}

// OperatorAddr is the address of operator
func (d *Delegate) OperatorAddr() address.Address {
	addr, err := address.FromString(d.OperatorAddrStr)
	if err != nil {
		log.L().Panic("Error when decoding the poll protocol operator address from string.", zap.Error(err))
	}
	return addr
}

// RewardAddr is the address of rewardee, which is allowed to be nil
func (d *Delegate) RewardAddr() address.Address {
	if d.RewardAddrStr == "" {
		return nil
	}
	addr, err := address.FromString(d.RewardAddrStr)
	if err != nil {
		log.L().Panic("Error when decoding the poll protocol rewardee address from string.", zap.Error(err))
	}
	return addr
}

// Votes returns the votes
func (d *Delegate) Votes() *big.Int {
	val, ok := big.NewInt(0).SetString(d.VotesStr, 10)
	if !ok {
		log.S().Panicf("Error when casting votes string %s into big int", d.VotesStr)
	}
	return val
}

// InitAdminAddr returns the address of the initial rewarding protocol admin
func (r *Rewarding) InitAdminAddr() address.Address {
	addr, err := address.FromString(r.InitAdminAddrStr)
	if err != nil {
		log.L().Panic("Error when decoding the rewarding protocol init admin address from string.", zap.Error(err))
	}
	return addr
}

// InitBalance returns the init balance of the rewarding fund
func (r *Rewarding) InitBalance() *big.Int {
	val, ok := big.NewInt(0).SetString(r.InitBalanceStr, 10)
	if !ok {
		log.S().Panicf("Error when casting init balance string %s into big int", r.InitBalanceStr)
	}
	return val
}

// BlockReward returns the block reward amount
func (r *Rewarding) BlockReward() *big.Int {
	val, ok := big.NewInt(0).SetString(r.BlockRewardStr, 10)
	if !ok {
		log.S().Panicf("Error when casting block reward string %s into big int", r.BlockRewardStr)
	}
	return val
}

// EpochReward returns the epoch reward amount
func (r *Rewarding) EpochReward() *big.Int {
	val, ok := big.NewInt(0).SetString(r.EpochRewardStr, 10)
	if !ok {
		log.S().Panicf("Error when casting epoch reward string %s into big int", r.EpochRewardStr)
	}
	return val
}
