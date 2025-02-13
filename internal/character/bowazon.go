package character

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/skill"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/game"
)

const (
	maxBowazonAttackLoops = 10
	minBowazonDistance    = 1
	maxBowazonDistance    = 30
)

type Bowazon struct {
	BaseCharacter
}

func aoeSkill() skill.ID {
	ctx := context.Get()

	ctx.Logger.Info("aoeSkill", slog.String("aoeSkill", ctx.CharacterCfg.Character.Bowazon.AoeSkill))

	if ctx.CharacterCfg.Character.Bowazon.AoeSkill == "explodingarrow" {
		return skill.ExplodingArrow
	} else {
		return skill.MultipleShot
	}
}

func (s Bowazon) AmmoNeeded() string {
	ctx := context.Get()

	ammoType := "Arrows"

	if ctx.CharacterCfg.Character.Bowazon.Ammo == "Bolts" {
		ammoType = "Bolts"
	}

	return ammoType
}

func (s Bowazon) CheckKeyBindings() []skill.ID {
	requireKeybindings := []skill.ID{aoeSkill(), skill.MagicArrow, skill.TomeOfTownPortal}
	missingKeybindings := []skill.ID{}

	for _, cskill := range requireKeybindings {
		if _, found := s.Data.KeyBindings.KeyBindingForSkill(cskill); !found {
			missingKeybindings = append(missingKeybindings, cskill)
		}
	}

	if len(missingKeybindings) > 0 {
		s.Logger.Debug("There are missing required key bindings.", slog.Any("Bindings", missingKeybindings))
	}

	bowFound := false
	crossbowFound := false

	ctx := context.Get()
	for _, i := range ctx.Data.Inventory.ByLocation(item.LocationEquipped) {
		itemType := i.Type()

		if itemType.IsType(item.TypeBow) || itemType.IsType(item.TypeAmazonBow) {
			bowFound = true
		} else if itemType.IsType(item.TypeCrossbow) {
			crossbowFound = true
		}
	}

	ammoNeeded := s.AmmoNeeded()

	// TODO: How can i raise an error to stop the bot? It won't be possible to refill ammo!
	if ammoNeeded == "Arrows" && !bowFound {
		s.Logger.Error("Bow not found and ammo type Arrows selected")
		missingKeybindings = append(missingKeybindings, skill.BaalTeleport)
	} else if ammoNeeded == "Bolts" && !crossbowFound {
		s.Logger.Error("Crossbow not found and ammo type Bolts selected")
		missingKeybindings = append(missingKeybindings, skill.BaalTeleport)
	}

	return missingKeybindings
}

func (s Bowazon) KillMonsterSequence(
	monsterSelector func(d game.Data) (data.UnitID, bool),
	skipOnImmunities []stat.Resist,
) error {
	completedAttackLoops := 0
	previousUnitID := 0
	const numOfAttacks = 5
	aoeSkill := aoeSkill()

	for {
		id, found := monsterSelector(*s.Data)
		if !found {
			return nil
		}
		if previousUnitID != int(id) {
			completedAttackLoops = 0
		}

		if !s.preBattleChecks(id, skipOnImmunities) {
			return nil
		}

		if completedAttackLoops >= maxBowazonAttackLoops {
			return nil
		}

		monster, found := s.Data.Monsters.FindByID(id)
		if !found {
			s.Logger.Info("Monster not found", slog.String("monster", fmt.Sprintf("%v", monster)))
			return nil
		}

		manaPercent := s.Data.PlayerUnit.MPPercent()

		// Low mana, do primary attack
		if manaPercent < 15 {
			step.PrimaryAttack(id, numOfAttacks, false, step.Distance(minBowazonDistance, maxBowazonDistance))
		} else {
			step.SecondaryAttack(aoeSkill, id, numOfAttacks, step.Distance(minBowazonDistance, maxBowazonDistance))
		}

		// Magic arrow 10% of the time until we can detect physical immune
		if manaPercent > 15 && rand.Float64() < 0.1 {
			step.SecondaryAttack(skill.MagicArrow, id, numOfAttacks, step.Distance(minBowazonDistance, maxBowazonDistance))
		}

		completedAttackLoops++
		previousUnitID = int(id)
	}
}

func (s Bowazon) killMonster(npc npc.ID, t data.MonsterType) error {
	return s.KillMonsterSequence(func(d game.Data) (data.UnitID, bool) {
		m, found := d.Monsters.FindOne(npc, t)
		if !found {
			return 0, false
		}

		return m.UnitID, true
	}, nil)
}

func (s Bowazon) killBoss(npc npc.ID, t data.MonsterType) error {
	return s.KillMonsterSequence(func(d game.Data) (data.UnitID, bool) {
		m, found := d.Monsters.FindOne(npc, t)
		if !found {
			return 0, false
		}

		return m.UnitID, true
	}, nil)
}

func (s Bowazon) PreCTABuffSkills() []skill.ID {
	if _, found := s.Data.KeyBindings.KeyBindingForSkill(skill.Valkyrie); found {
		return []skill.ID{skill.Valkyrie}
	} else {
		return []skill.ID{}
	}
}

func (s Bowazon) BuffSkills() []skill.ID {
	return []skill.ID{}
}

func (s Bowazon) KillCountess() error {
	return s.killMonster(npc.DarkStalker, data.MonsterTypeSuperUnique)
}

func (s Bowazon) KillAndariel() error {
	return s.killBoss(npc.Andariel, data.MonsterTypeUnique)
}

func (s Bowazon) KillSummoner() error {
	return s.killMonster(npc.Summoner, data.MonsterTypeUnique)
}

func (s Bowazon) KillDuriel() error {
	return s.killBoss(npc.Duriel, data.MonsterTypeUnique)
}

func (s Bowazon) KillCouncil() error {
	return s.KillMonsterSequence(func(d game.Data) (data.UnitID, bool) {
		// Exclude monsters that are not council members
		var councilMembers []data.Monster
		for _, m := range d.Monsters {
			if m.Name == npc.CouncilMember || m.Name == npc.CouncilMember2 || m.Name == npc.CouncilMember3 {
				councilMembers = append(councilMembers, m)
			}
		}

		// Order council members by distance
		sort.Slice(councilMembers, func(i, j int) bool {
			distanceI := s.PathFinder.DistanceFromMe(councilMembers[i].Position)
			distanceJ := s.PathFinder.DistanceFromMe(councilMembers[j].Position)

			return distanceI < distanceJ
		})

		for _, m := range councilMembers {
			return m.UnitID, true
		}

		return 0, false
	}, nil)
}

func (s Bowazon) KillMephisto() error {
	return s.killBoss(npc.Mephisto, data.MonsterTypeUnique)
}

func (s Bowazon) KillIzual() error {
	return s.killBoss(npc.Izual, data.MonsterTypeUnique)
}

func (s Bowazon) KillDiablo() error {
	timeout := time.Second * 20
	startTime := time.Now()
	diabloFound := false

	for {
		if time.Since(startTime) > timeout && !diabloFound {
			s.Logger.Error("Diablo was not found, timeout reached")
			return nil
		}

		diablo, found := s.Data.Monsters.FindOne(npc.Diablo, data.MonsterTypeUnique)
		if !found || diablo.Stats[stat.Life] <= 0 {
			// Already dead
			if diabloFound {
				return nil
			}

			// Keep waiting...
			time.Sleep(200)
			continue
		}

		diabloFound = true
		s.Logger.Info("Diablo detected, attacking")

		return s.killMonster(npc.Diablo, data.MonsterTypeUnique)
	}
}

func (s Bowazon) KillPindle() error {
	return s.killBoss(npc.DefiledWarrior, data.MonsterTypeSuperUnique)
}

func (s Bowazon) KillNihlathak() error {
	return s.killBoss(npc.Nihlathak, data.MonsterTypeSuperUnique)
}

func (s Bowazon) KillBaal() error {
	return s.killBoss(npc.BaalCrab, data.MonsterTypeUnique)
}
