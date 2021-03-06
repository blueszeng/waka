package hall

import (
	"math"
	"math/rand"
	"time"

	"github.com/sirupsen/logrus"
	linq "gopkg.in/ahmetb/go-linq.v3"

	"github.com/liuhan907/waka/waka-cow2/database"
	"github.com/liuhan907/waka/waka-cow2/modules/hall/tools"
	"github.com/liuhan907/waka/waka-cow2/modules/hall/tools/cow"
	"github.com/liuhan907/waka/waka-cow2/proto"
)

type payForAnotherPlayerRoundT struct {
	// 总分
	Points int32
	// 胜利的场数
	VictoriousNumber int32

	// 手牌
	Pokers []string

	// 最佳配牌
	BestPokers []string
	// 最佳牌型权重
	BestWeight int32
	// 最佳牌型
	BestPattern string
	// 最佳牌型倍率
	BestRate int32
	// 最佳得分
	BestPoints int32

	// 是否抢庄
	Grab bool
	// 抢庄已提交
	GrabCommitted bool
	// 倍率
	Rate int32
	// 倍率已提交
	RateCommitted bool

	// 阶段完成已提交
	ContinueWithCommitted bool

	// 本阶段消息是否已发送
	Sent bool
}

type payForAnotherPlayerT struct {
	Room *payForAnotherRoomT

	Player database.Player
	Pos    int32
	Ready  bool

	Round payForAnotherPlayerRoundT
}

func (player *payForAnotherPlayerT) NiuniuRoomData1PlayerData() (pb *cow_proto.NiuniuRoomData1_PlayerData) {
	lost := false
	if player, being := player.Room.Hall.players[player.Player]; !being || player.Remote == "" {
		lost = true
	}
	return &cow_proto.NiuniuRoomData1_PlayerData{
		Player: player.Room.Hall.ToPlayer(player.Player),
		Pos:    player.Pos,
		Ready:  player.Ready,
		Lost:   lost,
	}
}

type payForAnotherPlayerMapT map[database.Player]*payForAnotherPlayerT

func (players payForAnotherPlayerMapT) NiuniuRoomData1RoomPlayer() (pb []*cow_proto.NiuniuRoomData1_PlayerData) {
	for _, player := range players {
		pb = append(pb, player.NiuniuRoomData1PlayerData())
	}
	return pb
}

func (players payForAnotherPlayerMapT) ToSlice() (d []*payForAnotherPlayerT) {
	for _, player := range players {
		d = append(d, player)
	}
	return d
}

// ---------------------------------------------------------------------------------------------------------------------

type payForAnotherRoomT struct {
	Hall *actorT

	Id      int32
	Option  *cow_proto.NiuniuRoomOption
	Creator database.Player
	Owner   database.Player
	Players payForAnotherPlayerMapT

	loop func() bool
	tick func()

	Seats *tools.NumberPool

	Gaming      bool
	RoundNumber int32
	Step        string
	Banker      database.Player
	Bans        map[database.Player]bool

	Distribution []map[database.Player][]string
	King         []database.Player
}

// ---------------------------------------------------------------------------------------------------------------------

func (r *payForAnotherRoomT) CreateDiamonds() int32 {
	switch r.Option.GetRoundNumber() {
	case 12:
		return 4
	case 20:
		return 6
	default:
		return math.MaxInt32
	}
}

func (r *payForAnotherRoomT) EnterDiamonds() int32 {
	return 0
}

func (r *payForAnotherRoomT) CostDiamonds() int32 {
	return r.CreateDiamonds()
}

func (r *payForAnotherRoomT) GetId() int32 {
	return r.Id
}

func (r *payForAnotherRoomT) GetOption() *cow_proto.NiuniuRoomOption {
	return r.Option
}

func (r *payForAnotherRoomT) GetCreator() database.Player {
	return r.Creator
}

func (r *payForAnotherRoomT) GetOwner() database.Player {
	return r.Owner
}

func (r *payForAnotherRoomT) GetGaming() bool {
	return r.Gaming
}

func (r *payForAnotherRoomT) GetRoundNumber() int32 {
	return r.RoundNumber
}

func (r *payForAnotherRoomT) GetBanker() database.Player {
	return r.Banker
}

func (r *payForAnotherRoomT) GetPlayers() []database.Player {
	var d []database.Player
	linq.From(r.Players).SelectT(func(pair linq.KeyValue) database.Player { return pair.Key.(database.Player) }).ToSlice(&d)
	return d
}

func (r *payForAnotherRoomT) NiuniuRoomData1() *cow_proto.NiuniuRoomData1 {
	return &cow_proto.NiuniuRoomData1{
		Id:      r.Id,
		Option:  r.GetOption(),
		Creator: r.Hall.ToPlayer(r.Creator),
		Owner:   r.Hall.ToPlayer(r.Owner),
		Players: r.Players.NiuniuRoomData1RoomPlayer(),
		Gaming:  r.Gaming,
	}
}

func (r *payForAnotherRoomT) NiuniuRoundStatus(player database.Player) *cow_proto.NiuniuRoundStatus {
	var pokers []string
	var players []*cow_proto.NiuniuRoundStatus_PlayerData
	for id, playerData := range r.Players {
		players = append(players, &cow_proto.NiuniuRoundStatus_PlayerData{
			Id:            int32(id),
			Points:        playerData.Round.Points,
			GrabCommitted: playerData.Round.GrabCommitted,
			Grab:          playerData.Round.Grab,
			RateCommitted: playerData.Round.RateCommitted,
			Rate:          playerData.Round.Rate,
		})
		if playerData.Player == player {
			if r.Step == "round_clear" || r.Step == "round_finally" {
				pokers = playerData.Round.BestPokers
			} else {
				pokers = playerData.Round.Pokers[:4]
			}
		}
	}

	return &cow_proto.NiuniuRoundStatus{
		Step:        r.Step,
		RoomId:      r.Id,
		RoundNumber: r.RoundNumber,
		Players:     players,
		Banker:      int32(r.Banker),
		Pokers:      pokers,
	}
}

func (r *payForAnotherRoomT) NiuniuGrabAnimation() *cow_proto.NiuniuGrabAnimation {
	var players []*cow_proto.NiuniuGrabAnimation_PlayerData
	for _, player := range r.Players {
		players = append(players, &cow_proto.NiuniuGrabAnimation_PlayerData{
			PlayerId: int32(player.Player),
			Grab:     player.Round.Grab,
		})
	}
	return &cow_proto.NiuniuGrabAnimation{
		Players: players,
	}
}

func (r *payForAnotherRoomT) NiuniuRoundClear() *cow_proto.NiuniuRoundClear {
	var players []*cow_proto.NiuniuRoundClear_PlayerData
	for _, player := range r.Players {
		players = append(players, &cow_proto.NiuniuRoundClear_PlayerData{
			Player:     r.Hall.ToPlayer(player.Player),
			Points:     player.Round.Points,
			Pokers:     player.Round.BestPokers,
			Type:       player.Round.BestPattern,
			Rate:       player.Round.BestRate,
			ThisPoints: player.Round.BestPoints,
		})
	}
	return &cow_proto.NiuniuRoundClear{Players: players, FinallyAt: time.Now().Format("2006-01-02 15:04:05")}
}

func (r *payForAnotherRoomT) NiuniuRoundFinally() *cow_proto.NiuniuRoundFinally {
	var players []*cow_proto.NiuniuRoundFinally_PlayerData
	for _, player := range r.Players {
		players = append(players, &cow_proto.NiuniuRoundFinally_PlayerData{
			Player:    r.Hall.ToPlayer(player.Player),
			Points:    int32(player.Round.Points),
			Victories: player.Round.VictoriousNumber,
		})
	}
	return &cow_proto.NiuniuRoundFinally{Players: players, FinallyAt: time.Now().Format("2006-01-02 15:04:05")}
}

func (r *payForAnotherRoomT) BackendRoom() map[string]interface{} {
	var players []map[string]interface{}
	for _, player := range r.Players {
		playerData := player.Player.PlayerData()
		lost := false
		if playerData, being := r.Hall.players[player.Player]; !being || playerData.Remote == "" {
			lost = true
		}
		d := map[string]interface{}{
			"id":       player.Player,
			"nickname": playerData.Nickname,
			"head":     playerData.Head,
			"pos":      player.Pos,
			"ready":    player.Ready,
			"offline":  lost,
			"score":    player.Round.Points,
		}
		players = append(players, d)
	}
	return map[string]interface{}{
		"id":       r.Id,
		"option":   *r.Option,
		"creator":  r.Creator,
		"owner":    r.Owner,
		"players":  players,
		"rounding": r.RoundNumber,
		"status":   r.Step,
		"banker":   r.Banker,
	}
}

// ---------------------------------------------------------------------------------------------------------------------

func (r *payForAnotherRoomT) Loop() {
	for {
		if r.loop == nil {
			return
		}
		if !r.loop() {
			return
		}
	}
}

func (r *payForAnotherRoomT) Tick() {
	if r.tick != nil {
		r.tick()
	}
}

func (r *payForAnotherRoomT) Left(player *playerT) {
	if !r.Gaming {
		if roomPlayer, being := r.Players[player.Player]; being {
			delete(r.Players, player.Player)
			player.InsideCow = 0
			r.Seats.Return(roomPlayer.Pos)

			if r.Owner == player.Player {
				r.Owner = 0
				if len(r.Players) > 0 {
					for _, player := range r.Players {
						r.Owner = player.Player
						break
					}
				}
			}

			r.Hall.sendNiuniuUpdateRoomForAll(r)
		}
	}
}

func (r *payForAnotherRoomT) Recover(player *playerT) {
	if _, being := r.Players[player.Player]; being {
		r.Players[player.Player].Round.Sent = false
	}

	r.Hall.sendNiuniuUpdateRoomForAll(r)
	if r.Gaming {
		r.Hall.sendNiuniuUpdateRound(player.Player, r)
		r.Loop()
	}
}

func (r *payForAnotherRoomT) CreateRoom(hall *actorT, id int32, option *cow_proto.NiuniuRoomOption, creator database.Player) cowRoomT {
	*r = payForAnotherRoomT{
		Hall:    hall,
		Id:      id,
		Option:  option,
		Creator: creator,
		Players: make(payForAnotherPlayerMapT, 5),
		Seats:   tools.NewNumberPool(1, 5, false),
		Bans:    make(map[database.Player]bool),
	}

	if creator.PlayerData().Diamonds < r.CreateDiamonds() {
		r.Hall.sendNiuniuCreateRoomFailed(creator, 1)
		return nil
	} else {
		r.Hall.cowRooms[id] = r
		r.Hall.sendNiuniuRoomCreated(creator, id)
		return r
	}
}

func (r *payForAnotherRoomT) JoinRoom(player *playerT) {
	if r.Option.GetScret() {
		if !database.QueryPlayerCanJoin(r.Creator, player.Player) {
			r.Hall.sendNiuniuJoinRoomFailed(player.Player, 6)
			return
		}
	}

	if r.Bans[player.Player] {
		r.Hall.sendNiuniuJoinRoomFailed(player.Player, 5)
		return
	}

	_, being := r.Players[player.Player]
	if being {
		r.Hall.sendNiuniuJoinRoomFailed(player.Player, 2)
		return
	}

	if r.Gaming {
		r.Hall.sendNiuniuJoinRoomFailed(player.Player, 4)
		return
	}

	seat, has := r.Seats.Acquire()
	if !has {
		r.Hall.sendNiuniuJoinRoomFailed(player.Player, 0)
		return
	}

	r.Players[player.Player] = &payForAnotherPlayerT{
		Room:   r,
		Player: player.Player,
		Pos:    seat,
	}

	if r.Owner == 0 {
		r.Owner = player.Player
	}

	if player.Player.PlayerData().VictoryRate > 0 {
		r.King = append(r.King, player.Player)
	}

	player.InsideCow = r.Id

	r.Hall.sendNiuniuRoomJoined(player.Player, r)
	r.Hall.sendNiuniuUpdateRoomForAll(r)
}

func (r *payForAnotherRoomT) LeaveRoom(player *playerT) {
	if !r.Gaming {
		if roomPlayer, being := r.Players[player.Player]; being {
			player.InsideCow = 0
			delete(r.Players, player.Player)
			r.Seats.Return(roomPlayer.Pos)

			r.Hall.sendNiuniuRoomLeft(player.Player)

			if r.Owner == player.Player {
				r.Owner = 0
				if len(r.Players) > 0 {
					for _, player := range r.Players {
						r.Owner = player.Player
						break
					}
				}
			}

			r.Hall.sendNiuniuUpdateRoomForAll(r)
		}
	}
}

func (r *payForAnotherRoomT) SwitchReady(player *playerT) {
	if !r.Gaming {
		if roomPlayer, being := r.Players[player.Player]; being {
			roomPlayer.Ready = !roomPlayer.Ready
			r.Hall.sendNiuniuUpdateRoomForAll(r)
		}
	}
}

func (r *payForAnotherRoomT) Dismiss(player *playerT) {
	if !r.Gaming {
		if r.Creator == player.Player {
			delete(r.Hall.cowRooms, r.Id)
			for _, player := range r.Players {
				r.Hall.players[player.Player].InsideCow = 0
				r.Hall.sendNiuniuRoomLeftByDismiss(player.Player)
			}
		}
	}
}

func (r *payForAnotherRoomT) KickPlayer(player *playerT, target database.Player, ban bool) {
	if !r.Gaming {
		if r.Creator == player.Player || r.Owner == player.Player {
			if targetPlayer, being := r.Hall.players[target]; being {
				targetPlayer.InsideCow = 0
			}

			if targetPlayer, being := r.Players[target]; being {
				delete(r.Players, target)
				r.Seats.Return(targetPlayer.Pos)
				r.Hall.sendNiuniuRoomLeft(player.Player)

				if ban {
					r.Bans[target] = true
				}

				if r.Owner == player.Player {
					r.Owner = 0
					if len(r.Players) > 0 {
						for _, player := range r.Players {
							r.Owner = player.Player
							break
						}
					}
				}

				r.Hall.sendNiuniuUpdateRoomForAll(r)
			}
		}
	}
}

func (r *payForAnotherRoomT) Start(player *playerT) {
	if !r.Gaming {
		if r.Owner == player.Player {
			started := true
			for _, target := range r.Players {
				if !target.Ready {
					started = false
				}
			}
			if !started {
				return
			}

			err := database.CowPayForAnotherSettle(r.Id, &database.CowPlayerRoomCost{
				Player: r.Creator,
				Number: r.CostDiamonds(),
			})
			if err != nil {
				log.WithFields(logrus.Fields{
					"room_id": r.Id,
					"creator": r.Creator,
					"option":  r.Option.String(),
					"cost":    r.CostDiamonds(),
					"err":     err,
				}).Warnln("pay for another cost settle failed")
				return
			}

			r.Hall.sendPlayerSecret(r.Creator)

			r.loop = r.loopStart

			r.Loop()
		}
	}
}

func (r *payForAnotherRoomT) SpecifyBanker(player *playerT, banker database.Player) {
	if r.Gaming {
		if _, being := r.Players[banker]; being {
			r.Banker = banker

			r.Loop()
		}
	}
}

func (r *payForAnotherRoomT) Grab(player *playerT, grab bool) {
	if r.Gaming {
		r.Players[player.Player].Round.Grab = grab
		r.Players[player.Player].Round.GrabCommitted = true

		r.Hall.sendNiuniuUpdateRoundForAll(r)

		r.Loop()
	}
}

func (r *payForAnotherRoomT) SpecifyRate(player *playerT, rate int32) {
	if r.Gaming {
		r.Players[player.Player].Round.Rate = rate
		r.Players[player.Player].Round.RateCommitted = true

		r.Hall.sendNiuniuUpdateRoundForAll(r)

		r.Loop()
	}
}

func (r *payForAnotherRoomT) ContinueWith(player *playerT) {
	if r.Gaming {
		r.Players[player.Player].Round.ContinueWithCommitted = true

		r.Loop()
	}
}

func (r *payForAnotherRoomT) PostRoomMessage(player *playerT, content string) {
	if r.Players[player.Player] == nil {
		return
	}

	for _, target := range r.Players {
		if target.Player == player.Player {
			continue
		}

		r.Hall.sendNiuniuRoomMessage(target.Player, player.Player, content)
	}
}

// ---------------------------------------------------------------------------------------------------------------------

func (r *payForAnotherRoomT) loopStart() bool {
	var king database.Player
	for _, k := range r.King {
		if _, being := r.Players[k]; being {
			king = k
			break
		}
	}
	if king != 0 {
		var players []database.Player
		linq.From(r.Players).SelectT(func(x linq.KeyValue) database.Player {
			return x.Key.(database.Player)
		}).ToSlice(&players)
		r.Distribution = cow.Distributing(king, players, r.Option.GetRoundNumber(), king.PlayerData().VictoryRate, r.Option.GetMode(), r.Option.GetAdditionalPokers() == 1)
	}

	r.Gaming = true
	r.RoundNumber = 1

	r.Hall.sendNiuniuStartedForAll(r, r.RoundNumber)

	if r.Option.GetBankerMode() == 0 || r.Option.GetBankerMode() == 1 {
		r.loop = r.loopSpecifyBanker
	} else if r.Option.GetBankerMode() == 2 {
		r.loop = func() bool {
			return r.loopDeal4(r.loopGrab)
		}
	}

	return true
}

func (r *payForAnotherRoomT) loopSpecifyBanker() bool {
	r.Step = "require_specify_banker"
	for _, player := range r.Players {
		player.Round.Sent = false
	}

	r.loop = r.loopSpecifyBankerContinue
	r.tick = buildTickNumber(
		8,
		func(number int32) {
			r.Hall.sendNiuniuCountdownForAll(r, number)
		},
		func() {
			r.Banker = r.Owner
		},
		r.Loop,
	)
	return true
}

func (r *payForAnotherRoomT) loopSpecifyBankerContinue() bool {
	if r.Banker == 0 {
		for _, player := range r.Players {
			if !player.Round.Sent {
				r.Hall.sendNiuniuRequireSpecifyBanker(player.Player, player.Player == r.Owner)
				player.Round.Sent = true
			}
		}
		return false
	}

	r.tick = nil
	r.loop = func() bool {
		return r.loopDeal4(r.loopSpecifyRate)
	}

	return true
}

func (r *payForAnotherRoomT) loopDeal4(loop func() bool) bool {
	if r.Distribution == nil {
		pokers := cow.Acquire5(len(r.Players))
		i := 0
		for _, player := range r.Players {
			player.Round.Pokers = pokers[i]
			i++
		}
	} else {
		roundPokers := r.Distribution[r.RoundNumber-1]
		for _, player := range r.Players {
			player.Round.Pokers = roundPokers[player.Player]
		}
	}

	for _, player := range r.Players {
		player.Round.BestPokers, player.Round.BestWeight, player.Round.BestPattern, player.Round.BestRate, _ =
			cow.SearchBestPokerPattern(player.Round.Pokers, r.Option.GetMode(), r.Option.GetAdditionalPokers() == 1)
		r.Hall.sendNiuniuDeal4(player.Player, player.Round.Pokers[:4])
	}

	r.Hall.sendNiuniuUpdateRoundForAll(r)

	r.loop = loop

	return true
}

func (r *payForAnotherRoomT) loopGrab() bool {
	r.Step = "require_grab"
	for _, player := range r.Players {
		player.Round.Sent = false
	}

	r.Hall.sendNiuniuUpdateRoundForAll(r)

	r.loop = r.loopGrabContinue
	r.tick = buildTickNumber(
		13,
		func(number int32) {
			r.Hall.sendNiuniuCountdownForAll(r, number)
		},
		func() {
			for _, player := range r.Players {
				if !player.Round.GrabCommitted {
					player.Round.Grab = false
					player.Round.GrabCommitted = true
				}
			}
		},
		r.Loop,
	)

	return true
}

func (r *payForAnotherRoomT) loopGrabContinue() bool {
	finally := true
	for _, player := range r.Players {
		if !player.Round.GrabCommitted {
			finally = false
			if !player.Round.Sent {
				r.Hall.sendNiuniuRequireGrab(player.Player)
				player.Round.Sent = true
			}
		}
	}

	if !finally {
		return false
	}

	r.tick = nil
	r.loop = r.loopGrabAnimation

	return true
}

func (r *payForAnotherRoomT) loopGrabAnimation() bool {
	r.Step = "grab_animation"
	for _, player := range r.Players {
		player.Round.Sent = false
		player.Round.ContinueWithCommitted = false
	}

	r.Hall.sendNiuniuUpdateRoundForAll(r)

	r.loop = r.loopGrabAnimationContinue
	r.tick = buildTickNumber(
		8,
		func(number int32) {
			r.Hall.sendNiuniuCountdownForAll(r, number)
		},
		func() {
			for _, player := range r.Players {
				player.Round.ContinueWithCommitted = true
			}
		},
		r.Loop,
	)

	return true
}

func (r *payForAnotherRoomT) loopGrabAnimationContinue() bool {
	finally := true
	for _, player := range r.Players {
		if !player.Round.ContinueWithCommitted {
			finally = false
			if !player.Round.Sent {
				r.Hall.sendNiuniuGrabAnimation(player.Player, r)
				player.Round.Sent = true
			}
		}
	}

	if !finally {
		return false
	}

	r.tick = nil
	r.loop = r.loopGrabSelect

	return true
}

func (r *payForAnotherRoomT) loopGrabSelect() bool {
	var candidates []database.Player
	for _, player := range r.Players {
		if player.Round.Grab {
			candidates = append(candidates, player.Player)
		}
	}

	if len(candidates) > 0 {
		r.Banker = candidates[rand.Int()%len(candidates)]

		log.WithFields(logrus.Fields{
			"candidates": candidates,
			"banker":     r.Banker,
		}).Debugln("grab")
	} else {
		r.Banker = r.Owner

		log.WithFields(logrus.Fields{
			"banker": r.Banker,
		}).Debugln("no player grab")
	}

	r.Hall.sendNiuniuUpdateRoundForAll(r)

	r.loop = r.loopSpecifyRate

	return true
}

func (r *payForAnotherRoomT) loopSpecifyRate() bool {
	r.Step = "require_specify_rate"
	for _, player := range r.Players {
		player.Round.Sent = false
		if player.Player == r.Banker {
			player.Round.Rate = 1
			player.Round.RateCommitted = true
		}
	}

	r.Hall.sendNiuniuUpdateRoundForAll(r)

	r.loop = r.loopSpecifyRateContinue
	r.tick = buildTickNumber(
		5,
		func(number int32) {
			r.Hall.sendNiuniuCountdownForAll(r, number)
		},
		func() {
			for _, player := range r.Players {
				if !player.Round.RateCommitted {
					player.Round.Rate = 1
					player.Round.RateCommitted = true
				}
			}
		},
		r.Loop,
	)

	return true
}

func (r *payForAnotherRoomT) loopSpecifyRateContinue() bool {
	finally := true
	for _, player := range r.Players {
		if !player.Round.RateCommitted {
			finally = false
			if !player.Round.Sent {
				r.Hall.sendNiuniuRequireSpecifyRate(player.Player, player.Player != r.Banker)
				player.Round.Sent = true
			}
		}
	}

	if !finally {
		return false
	}

	r.tick = nil
	r.loop = r.loopCommitConfirm

	return true
}

func (r *payForAnotherRoomT) loopCommitConfirm() bool {
	r.Step = "commit_confirm"
	for _, player := range r.Players {
		player.Round.Sent = false
		player.Round.ContinueWithCommitted = false
	}

	r.Hall.sendNiuniuUpdateRoundForAll(r)

	r.loop = r.loopCommitConfirmContinue
	r.tick = buildTickNumber(
		8,
		func(number int32) {
			r.Hall.sendNiuniuCountdownForAll(r, number)
		},
		func() {
			for _, player := range r.Players {
				player.Round.ContinueWithCommitted = true
			}
		},
		r.Loop,
	)

	return true
}

func (r *payForAnotherRoomT) loopCommitConfirmContinue() bool {
	finally := true
	for _, player := range r.Players {
		if !player.Round.ContinueWithCommitted {
			finally = false
			if !player.Round.Sent {
				r.Hall.sendNiuniuRequireCommitConfirm(player.Player, player.Round.Pokers)
				player.Round.Sent = true
			}
		}
	}

	if !finally {
		return false
	}

	r.tick = nil
	r.loop = r.loopSettle

	return true
}

func (r *payForAnotherRoomT) loopSettle() bool {
	if r.Players[r.Banker] == nil {
		for _, player := range r.Players {
			r.Banker = player.Player
			break
		}
	}

	banker := r.Players[r.Banker]
	playersMes := []*cow_proto.NiuniuRoundPlayerMes{}
	var players []*payForAnotherPlayerT
	for _, player := range r.Players {
		if player.Player != r.Banker {
			players = append(players, player)
		}
	}

	for _, player := range players {
		var applyRate int32
		var applySign int32
		if banker.Round.BestWeight >= player.Round.BestWeight {
			applyRate = int32(banker.Round.BestRate)
			applySign = -1
			banker.Round.VictoriousNumber++
		} else {
			applyRate = int32(player.Round.BestRate)
			applySign = 1
			player.Round.VictoriousNumber++
		}

		bs := r.Option.GetScore() * player.Round.Rate * applyRate * applySign * (-1)
		ps := r.Option.GetScore() * player.Round.Rate * applyRate * applySign

		banker.Round.BestPoints += bs
		player.Round.BestPoints += ps

		banker.Round.Points += bs
		player.Round.Points += ps
		playersMes = append(playersMes, &cow_proto.NiuniuRoundPlayerMes{
			Player: int32(player.Player.PlayerData().Id),
			Point:  ps,
		})
	}
	r.Hall.sendNiuniuRoundForAll(r.Banker, r.GetBanker(), playersMes, r)
	r.loop = r.loopRoundClear

	return true
}

func (r *payForAnotherRoomT) loopRoundClear() bool {
	r.Step = "round_clear"
	for _, player := range r.Players {
		player.Round.Sent = false
		player.Round.ContinueWithCommitted = false
	}

	r.Hall.sendNiuniuUpdateRoundForAll(r)

	r.loop = r.loopRoundClearContinue
	r.tick = buildTickNumber(
		8,
		func(number int32) {
		},
		func() {
			for _, player := range r.Players {
				player.Round.ContinueWithCommitted = true
			}
		},
		r.Loop,
	)

	return true
}

func (r *payForAnotherRoomT) loopRoundClearContinue() bool {
	finally := true
	for _, player := range r.Players {
		if !player.Round.ContinueWithCommitted {
			finally = false
			if !player.Round.Sent {
				r.Hall.sendNiuniuRoundClear(player.Player, r)
				player.Round.Sent = true
			}
		}
	}

	if !finally {
		return false
	}

	r.tick = nil
	r.loop = r.loopSelect

	return true
}

func (r *payForAnotherRoomT) loopSelect() bool {
	if r.RoundNumber < r.Option.GetRoundNumber() {
		r.loop = r.loopTransfer
	} else {
		r.loop = r.loopFinally
	}
	return true
}

func (r *payForAnotherRoomT) loopTransfer() bool {
	r.RoundNumber++
	if r.Option.GetBankerMode() == 1 {
		players := r.Players.ToSlice()
		/*		sort.Slice(players, func(i, j int) bool {
					return players[i].Pos < players[j].Pos
				})
				for i, player := range players {
					if player.Player == r.Banker {
						if i < len(players)-1 {
							r.Banker = players[i+1].Player
						} else {
							r.Banker = players[0].Player
						}
					}
				}*/
		index := r.Players[r.GetBanker()].Pos
	A:
		for {
			if index == 1 {
				index = 5
			} else {
				index -= 1
			}

			for _, player := range players {
				if player.Pos == index {

					r.Banker = player.Player
					break A
				}
			}
		}
		r.Hall.sendNiuniuUpdateRoundForAll(r)
	} else if r.Option.GetBankerMode() == 2 {
		r.Banker = 0
	}
	for _, player := range r.Players {
		player.Round = payForAnotherPlayerRoundT{
			Points:           player.Round.Points,
			VictoriousNumber: player.Round.VictoriousNumber,
		}
	}

	r.Hall.sendNiuniuStartedForAll(r, r.RoundNumber)

	if r.Option.GetBankerMode() == 0 || r.Option.GetBankerMode() == 1 {
		r.loop = func() bool {
			return r.loopDeal4(r.loopSpecifyRate)
		}
	} else if r.Option.GetBankerMode() == 2 {
		r.loop = func() bool {
			return r.loopDeal4(r.loopGrab)
		}
	}

	return true
}

func (r *payForAnotherRoomT) loopFinally() bool {
	r.Step = "round_finally"
	for _, player := range r.Players {
		player.Round.Sent = false
		player.Round.ContinueWithCommitted = false
	}

	r.Hall.sendNiuniuUpdateRoundForAll(r)

	r.loop = r.loopFinallyContinue
	r.tick = buildTickNumber(
		8,
		func(number int32) {
		},
		func() {
			for _, player := range r.Players {
				player.Round.ContinueWithCommitted = true
			}
		},
		r.Loop,
	)

	for _, player := range r.Players {
		if err := database.CowAddPayForAnotherWarHistory(player.Player, r.Id, r.NiuniuRoundFinally()); err != nil {
			log.WithFields(logrus.Fields{
				"err": err,
			}).Warnln("add cow player history failed")
		}
	}

	return true
}

func (r *payForAnotherRoomT) loopFinallyContinue() bool {
	finally := true
	for _, player := range r.Players {
		if !player.Round.ContinueWithCommitted {
			finally = false
			if !player.Round.Sent {
				r.Hall.sendNiuniuRoundFinally(player.Player, r)
				player.Round.Sent = true
			}
		}
	}

	if !finally {
		return false
	}

	r.loop = r.loopClean

	return true
}

func (r *payForAnotherRoomT) loopClean() bool {
	for _, player := range r.Players {
		if playerData, being := r.Hall.players[player.Player]; being {
			playerData.InsideCow = 0
		}
	}
	delete(r.Hall.cowRooms, r.Id)

	return false
}
