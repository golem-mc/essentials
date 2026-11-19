package core

import (
	"strconv"

	"github.com/golem-mc/golem/plugin"
	"github.com/shopspring/decimal"
)

// Trade is a price or a payout: money, an item stack or experience points.
// Essentials: Trade
type Trade struct {
	Money *decimal.Decimal
	Item  *plugin.ItemStack
	Exp   int
}

// MoneyTrade answers a trade of amount.
func MoneyTrade(amount decimal.Decimal) *Trade { return &Trade{Money: &amount} }

// CheckAffordable answers notEnoughMoney, missingItems or notEnoughExperience
// when the user cannot pay. Essentials: isAffordableFor
func (t *Trade) CheckAffordable(u *User) error {
	if t == nil {
		return nil
	}
	if t.Money != nil && t.Money.Sign() > 0 && !u.CanAfford(*t.Money) {
		return Tlf("notEnoughMoney")
	}
	if t.Item != nil && !t.Item.Empty() {
		p, ok := u.Player()
		if !ok || p.Inventory().Count(t.Item.Item) < t.Item.Count {
			return Tlf("missingItems", t.Item.Count, u.ess.ItemName(*t.Item))
		}
	}
	if t.Exp > 0 {
		p, ok := u.Player()
		if !ok || TotalExp(p) < t.Exp {
			return Tlf("notEnoughExperience")
		}
	}
	return nil
}

// Charge takes the price from the user. Essentials: charge
func (t *Trade) Charge(u *User) error {
	if t == nil {
		return nil
	}
	if err := t.CheckAffordable(u); err != nil {
		return err
	}
	if t.Money != nil && t.Money.Sign() > 0 {
		if err := u.SetMoney(u.Money().Sub(*t.Money)); err != nil {
			return err
		}
	}
	if p, ok := u.Player(); ok {
		if t.Item != nil && !t.Item.Empty() {
			p.Inventory().Remove(*t.Item)
		}
		if t.Exp > 0 {
			SetTotalExperience(p, TotalExp(p)-t.Exp)
		}
	}
	return nil
}

// Pay gives the trade to the user; items that do not fit are dropped at
// their feet. Essentials: pay
func (t *Trade) Pay(u *User) error {
	if t == nil {
		return nil
	}
	if t.Money != nil && t.Money.Sign() > 0 {
		if err := u.SetMoney(u.Money().Add(*t.Money)); err != nil {
			return err
		}
	}
	p, ok := u.Player()
	if !ok {
		return nil
	}
	if t.Item != nil && !t.Item.Empty() {
		u.ess.GiveItems(p, true, *t.Item)
	}
	if t.Exp > 0 {
		p.GiveExperience(t.Exp)
	}
	return nil
}

// Describe answers the trade for messages: the money formatted, the items as
// "count name", or the experience points.
func (t *Trade) Describe(e *Essentials) string {
	switch {
	case t == nil:
		return ""
	case t.Money != nil:
		return e.Economy.Format(*t.Money)
	case t.Item != nil:
		return strconv.Itoa(t.Item.Count) + " " + e.ItemName(*t.Item)
	default:
		return strconv.Itoa(t.Exp) + " exp"
	}
}

// TotalExp answers the player's current experience points, from level and
// progress: Player.TotalExperience is the score's running total, which
// spending levels does not lower. Essentials: SetExpFix.getTotalExperience
func TotalExp(p plugin.Player) int { return ExpAtLevel(p.Level()) + p.Experience() }

// SetTotalExperience sets the player's experience to total points.
// Essentials: SetExpFix.setTotalExperience
func SetTotalExperience(p plugin.Player, total int) {
	total = max(total, 0)
	p.SetTotalExperience(total)
}

// ExpToNextLevel answers the points from level to level+1.
// vanilla: Player.getXpNeededForNextLevel
func ExpToNextLevel(level int) int {
	switch {
	case level >= 30:
		return 112 + (level-30)*9
	case level >= 15:
		return 37 + (level-15)*5
	default:
		return 7 + level*2
	}
}

// ExpAtLevel answers the points needed to reach level from zero.
func ExpAtLevel(level int) int {
	n := 0
	for l := range level {
		n += ExpToNextLevel(l)
	}
	return n
}
