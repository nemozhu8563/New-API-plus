package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	StripePaymentTargetTopUp        = "topup"
	StripePaymentTargetSubscription = "subscription"
	StripeAdjustmentRefund          = "refund"
	StripeAdjustmentDispute         = "dispute"
)

var (
	ErrStripePaymentUnmanaged   = errors.New("stripe payment is not managed by this service")
	ErrStripeAdjustmentMismatch = errors.New("stripe payment adjustment does not match the original payment")
)

// StripePaymentReference binds a Stripe PaymentIntent or Charge to the local
// entitlement created by that payment. Nullable provider IDs keep the unique
// indexes portable across SQLite, MySQL, and PostgreSQL.
type StripePaymentReference struct {
	Id              int     `json:"id"`
	PaymentIntentId *string `json:"payment_intent_id,omitempty" gorm:"type:varchar(255);uniqueIndex:idx_stripe_payment_reference_intent,priority:1"`
	ChargeId        *string `json:"charge_id,omitempty" gorm:"type:varchar(255);uniqueIndex:idx_stripe_payment_reference_charge,priority:1"`
	Livemode        bool    `json:"livemode" gorm:"not null;uniqueIndex:idx_stripe_payment_reference_intent,priority:2;uniqueIndex:idx_stripe_payment_reference_charge,priority:2"`
	TargetKind      string  `json:"target_kind" gorm:"type:varchar(32);not null;index:idx_stripe_payment_reference_target,priority:1"`
	TargetId        int     `json:"target_id" gorm:"not null;index:idx_stripe_payment_reference_target,priority:2"`
	UserId          int     `json:"user_id" gorm:"not null;index"`
	AmountMinor     int64   `json:"amount_minor" gorm:"type:bigint;not null"`
	Currency        string  `json:"currency" gorm:"type:varchar(8);not null"`
	CreatedAt       int64   `json:"created_at" gorm:"type:bigint;not null"`
}

// StripePaymentRecovery stores the current entitlement effect for one local
// payment target. Keeping debt-paid quota separate allows a later dispute win
// or failed refund to restore quota correctly even after a newer top-up paid
// the debt.
type StripePaymentRecovery struct {
	Id                   int    `json:"id"`
	TargetKind           string `json:"target_kind" gorm:"type:varchar(32);not null;uniqueIndex:idx_stripe_payment_recovery_target,priority:1"`
	TargetId             int    `json:"target_id" gorm:"not null;uniqueIndex:idx_stripe_payment_recovery_target,priority:2"`
	UserId               int    `json:"user_id" gorm:"not null;index"`
	OriginalAmountMinor  int64  `json:"original_amount_minor" gorm:"type:bigint;not null"`
	OriginalQuota        int64  `json:"original_quota" gorm:"type:bigint;not null"`
	Currency             string `json:"currency" gorm:"type:varchar(8);not null"`
	Livemode             bool   `json:"livemode" gorm:"not null"`
	RecoveredQuota       int64  `json:"recovered_quota" gorm:"type:bigint;not null;default:0"`
	WalletRecoveredQuota int64  `json:"wallet_recovered_quota" gorm:"type:bigint;not null;default:0"`
	OutstandingQuota     int64  `json:"outstanding_quota" gorm:"type:bigint;not null;default:0;index"`
	DebtPaidQuota        int64  `json:"debt_paid_quota" gorm:"type:bigint;not null;default:0"`
	EntitlementRevoked   bool   `json:"entitlement_revoked" gorm:"not null"`
	CreatedAt            int64  `json:"created_at" gorm:"type:bigint;not null"`
	UpdatedAt            int64  `json:"updated_at" gorm:"type:bigint;not null"`
}

type StripePaymentAdjustment struct {
	Id                 int    `json:"id"`
	ObjectType         string `json:"object_type" gorm:"type:varchar(16);not null;uniqueIndex:idx_stripe_adjustment_object,priority:1"`
	ObjectId           string `json:"object_id" gorm:"type:varchar(255);not null;uniqueIndex:idx_stripe_adjustment_object,priority:2"`
	Livemode           bool   `json:"livemode" gorm:"not null;uniqueIndex:idx_stripe_adjustment_object,priority:3"`
	PaymentReferenceId int    `json:"payment_reference_id" gorm:"not null;index"`
	TargetKind         string `json:"target_kind" gorm:"type:varchar(32);not null;index:idx_stripe_adjustment_target,priority:1"`
	TargetId           int    `json:"target_id" gorm:"not null;index:idx_stripe_adjustment_target,priority:2"`
	EventId            string `json:"event_id" gorm:"type:varchar(255);not null"`
	PaymentIntentId    string `json:"payment_intent_id" gorm:"type:varchar(255);not null;default:''"`
	ChargeId           string `json:"charge_id" gorm:"type:varchar(255);not null;default:''"`
	AmountMinor        int64  `json:"amount_minor" gorm:"type:bigint;not null"`
	Currency           string `json:"currency" gorm:"type:varchar(8);not null"`
	Status             string `json:"status" gorm:"type:varchar(64);not null"`
	Active             bool   `json:"active" gorm:"not null;index"`
	EventCreated       int64  `json:"event_created" gorm:"type:bigint;not null"`
	EventPriority      int    `json:"event_priority" gorm:"not null"`
	CreatedAt          int64  `json:"created_at" gorm:"type:bigint;not null"`
	UpdatedAt          int64  `json:"updated_at" gorm:"type:bigint;not null"`
}

type StripePaymentSnapshot struct {
	PaymentIntentId string
	ChargeId        string
	AmountMinor     int64
}

type StripePaymentAdjustmentInput struct {
	ObjectType      string
	ObjectId        string
	EventId         string
	PaymentIntentId string
	ChargeId        string
	AmountMinor     int64
	Currency        string
	Livemode        bool
	Status          string
	Active          bool
	EventCreated    int64
	EventPriority   int
}

type StripePaymentAdjustmentResult struct {
	Managed          bool
	UserId           int
	TargetKind       string
	TargetId         int
	RecoveredQuota   int64
	OutstandingQuota int64
}

func normalizeStripeProviderId(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func findStripePaymentReferenceTx(tx *gorm.DB, paymentIntentId string, chargeId string, livemode bool) (*StripePaymentReference, error) {
	if tx == nil {
		tx = DB
	}
	paymentIntentId = strings.TrimSpace(paymentIntentId)
	chargeId = strings.TrimSpace(chargeId)
	if paymentIntentId == "" && chargeId == "" {
		return nil, ErrStripePaymentUnmanaged
	}
	query := tx.Where("livemode = ?", livemode)
	if paymentIntentId != "" && chargeId != "" {
		query = query.Where("payment_intent_id = ? OR charge_id = ?", paymentIntentId, chargeId)
	} else if paymentIntentId != "" {
		query = query.Where("payment_intent_id = ?", paymentIntentId)
	} else {
		query = query.Where("charge_id = ?", chargeId)
	}
	var refs []StripePaymentReference
	if err := query.Limit(2).Find(&refs).Error; err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, ErrStripePaymentUnmanaged
	}
	if len(refs) > 1 && refs[0].Id != refs[1].Id {
		return nil, fmt.Errorf("%w: provider IDs resolve to different payments", ErrStripeAdjustmentMismatch)
	}
	return &refs[0], nil
}

func registerStripePaymentReferenceTx(tx *gorm.DB, ref StripePaymentReference, originalAmountMinor int64, originalQuota int64) error {
	if tx == nil || ref.UserId <= 0 || ref.TargetId <= 0 ||
		(ref.TargetKind != StripePaymentTargetTopUp && ref.TargetKind != StripePaymentTargetSubscription) ||
		ref.AmountMinor <= 0 || originalAmountMinor <= 0 || originalQuota < 0 || strings.TrimSpace(ref.Currency) == "" ||
		(ref.PaymentIntentId == nil && ref.ChargeId == nil) {
		return fmt.Errorf("%w: invalid payment reference", ErrStripeAdjustmentMismatch)
	}
	now := common.GetTimestamp()
	state := StripePaymentRecovery{
		TargetKind: ref.TargetKind, TargetId: ref.TargetId, UserId: ref.UserId,
		OriginalAmountMinor: originalAmountMinor, OriginalQuota: originalQuota,
		Currency: strings.ToUpper(ref.Currency), Livemode: ref.Livemode,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&state).Error; err != nil {
		return err
	}
	var storedState StripePaymentRecovery
	if err := tx.Where("target_kind = ? AND target_id = ?", ref.TargetKind, ref.TargetId).First(&storedState).Error; err != nil {
		return err
	}
	if storedState.UserId != ref.UserId || storedState.OriginalAmountMinor != originalAmountMinor ||
		storedState.OriginalQuota != originalQuota || !strings.EqualFold(storedState.Currency, ref.Currency) ||
		storedState.Livemode != ref.Livemode {
		return fmt.Errorf("%w: payment recovery snapshot changed", ErrStripeAdjustmentMismatch)
	}

	ref.Currency = strings.ToUpper(ref.Currency)
	ref.CreatedAt = now
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ref).Error; err != nil {
		return err
	}
	storedRef, err := findStripePaymentReferenceTx(tx, valueOrEmpty(ref.PaymentIntentId), valueOrEmpty(ref.ChargeId), ref.Livemode)
	if err != nil {
		return err
	}
	if storedRef.TargetKind != ref.TargetKind || storedRef.TargetId != ref.TargetId ||
		storedRef.UserId != ref.UserId || storedRef.AmountMinor != ref.AmountMinor ||
		!strings.EqualFold(storedRef.Currency, ref.Currency) {
		return fmt.Errorf("%w: provider payment already belongs to another entitlement", ErrStripeAdjustmentMismatch)
	}
	return nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func registerStripeTopUpPaymentTx(tx *gorm.DB, topUp *TopUp, settlement StripeTopUpSettlement) error {
	if topUp == nil {
		return errors.New("topup is nil")
	}
	return registerStripePaymentReferenceTx(tx, StripePaymentReference{
		PaymentIntentId: normalizeStripeProviderId(settlement.PaymentIntentId),
		ChargeId:        normalizeStripeProviderId(settlement.ChargeId),
		Livemode:        settlement.Livemode,
		TargetKind:      StripePaymentTargetTopUp,
		TargetId:        topUp.Id,
		UserId:          topUp.UserId,
		AmountMinor:     settlement.AmountMinor,
		Currency:        settlement.Currency,
	}, settlement.AmountMinor, topUp.CreditedQuota)
}

func registerStripeSubscriptionPaymentsTx(tx *gorm.DB, settlement *StripeSubscriptionSettlement, sub *UserSubscription, payments []StripePaymentSnapshot) error {
	if tx == nil || settlement == nil || sub == nil || len(payments) == 0 {
		return fmt.Errorf("%w: subscription invoice has no payment reference", ErrStripeAdjustmentMismatch)
	}
	var paid int64
	for _, payment := range payments {
		if payment.AmountMinor <= 0 || (strings.TrimSpace(payment.PaymentIntentId) == "" && strings.TrimSpace(payment.ChargeId) == "") {
			return fmt.Errorf("%w: invalid subscription invoice payment", ErrStripeAdjustmentMismatch)
		}
		if paid > settlement.AmountPaidMinor-payment.AmountMinor {
			return fmt.Errorf("%w: subscription invoice payment total overflow", ErrStripeAdjustmentMismatch)
		}
		paid += payment.AmountMinor
		if err := registerStripePaymentReferenceTx(tx, StripePaymentReference{
			PaymentIntentId: normalizeStripeProviderId(payment.PaymentIntentId),
			ChargeId:        normalizeStripeProviderId(payment.ChargeId),
			Livemode:        settlement.Livemode,
			TargetKind:      StripePaymentTargetSubscription,
			TargetId:        settlement.Id,
			UserId:          sub.UserId,
			AmountMinor:     payment.AmountMinor,
			Currency:        settlement.Currency,
		}, settlement.AmountPaidMinor, sub.AmountTotal); err != nil {
			return err
		}
	}
	if paid != settlement.AmountPaidMinor {
		return fmt.Errorf("%w: subscription invoice payments do not equal amount paid", ErrStripeAdjustmentMismatch)
	}
	return nil
}

func backfillStripeTopUpPaymentReference(paymentIntentId string, chargeId string, livemode bool) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		query := lockForUpdate(tx).Where("payment_provider = ? AND status = ? AND provider_livemode = ?", PaymentProviderStripe, common.TopUpStatusSuccess, livemode)
		if paymentIntentId != "" && chargeId != "" {
			query = query.Where("provider_payment_intent = ? OR provider_charge_id = ?", paymentIntentId, chargeId)
		} else if paymentIntentId != "" {
			query = query.Where("provider_payment_intent = ?", paymentIntentId)
		} else if chargeId != "" {
			query = query.Where("provider_charge_id = ?", chargeId)
		} else {
			return ErrStripePaymentUnmanaged
		}
		var topUp TopUp
		if err := query.First(&topUp).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStripePaymentUnmanaged
			}
			return err
		}
		return registerStripeTopUpPaymentTx(tx, &topUp, StripeTopUpSettlement{
			CustomerId: topUp.ProviderCustomerId, PaymentIntentId: topUp.ProviderPaymentIntent,
			ChargeId: topUp.ProviderChargeId, AmountMinor: topUp.ExpectedAmountMinor,
			Currency: topUp.ExpectedCurrency, Livemode: topUp.ProviderLivemode,
		})
	})
}

func FindStripePaymentReference(paymentIntentId string, chargeId string, livemode bool) (*StripePaymentReference, error) {
	ref, err := findStripePaymentReferenceTx(DB, paymentIntentId, chargeId, livemode)
	if err == nil || !errors.Is(err, ErrStripePaymentUnmanaged) {
		return ref, err
	}
	if backfillErr := backfillStripeTopUpPaymentReference(paymentIntentId, chargeId, livemode); backfillErr != nil {
		return nil, backfillErr
	}
	return findStripePaymentReferenceTx(DB, paymentIntentId, chargeId, livemode)
}

func ApplyStripePaymentAdjustment(input StripePaymentAdjustmentInput) (*StripePaymentAdjustmentResult, error) {
	input.ObjectType = strings.TrimSpace(input.ObjectType)
	input.ObjectId = strings.TrimSpace(input.ObjectId)
	input.EventId = strings.TrimSpace(input.EventId)
	input.PaymentIntentId = strings.TrimSpace(input.PaymentIntentId)
	input.ChargeId = strings.TrimSpace(input.ChargeId)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.Status = strings.TrimSpace(input.Status)
	if (input.ObjectType != StripeAdjustmentRefund && input.ObjectType != StripeAdjustmentDispute) ||
		input.ObjectId == "" || input.EventId == "" || input.AmountMinor <= 0 || input.Currency == "" ||
		input.EventCreated <= 0 || input.EventPriority <= 0 {
		return nil, fmt.Errorf("%w: invalid adjustment", ErrStripeAdjustmentMismatch)
	}
	ref, err := FindStripePaymentReference(input.PaymentIntentId, input.ChargeId, input.Livemode)
	if err != nil {
		return nil, err
	}
	result := &StripePaymentAdjustmentResult{Managed: true, UserId: ref.UserId, TargetKind: ref.TargetKind, TargetId: ref.TargetId}
	groupChanged := false
	err = DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Where("id = ?", ref.UserId).First(&user).Error; err != nil {
			return err
		}
		var lockedRef StripePaymentReference
		if err := lockForUpdate(tx).Where("id = ?", ref.Id).First(&lockedRef).Error; err != nil {
			return err
		}
		if lockedRef.UserId != ref.UserId || lockedRef.TargetKind != ref.TargetKind || lockedRef.TargetId != ref.TargetId ||
			lockedRef.Livemode != input.Livemode || lockedRef.AmountMinor < input.AmountMinor ||
			!strings.EqualFold(lockedRef.Currency, input.Currency) {
			return fmt.Errorf("%w: adjustment differs from payment reference", ErrStripeAdjustmentMismatch)
		}
		if lockedRef.PaymentIntentId != nil && input.PaymentIntentId != "" && *lockedRef.PaymentIntentId != input.PaymentIntentId {
			return fmt.Errorf("%w: PaymentIntent mismatch", ErrStripeAdjustmentMismatch)
		}
		if lockedRef.ChargeId != nil && input.ChargeId != "" && *lockedRef.ChargeId != input.ChargeId {
			return fmt.Errorf("%w: Charge mismatch", ErrStripeAdjustmentMismatch)
		}
		if lockedRef.PaymentIntentId == nil && input.PaymentIntentId != "" {
			lockedRef.PaymentIntentId = normalizeStripeProviderId(input.PaymentIntentId)
		}
		if lockedRef.ChargeId == nil && input.ChargeId != "" {
			lockedRef.ChargeId = normalizeStripeProviderId(input.ChargeId)
		}
		if err := tx.Save(&lockedRef).Error; err != nil {
			return err
		}

		var recovery StripePaymentRecovery
		if err := lockForUpdate(tx).Where("target_kind = ? AND target_id = ?", ref.TargetKind, ref.TargetId).First(&recovery).Error; err != nil {
			return err
		}
		if recovery.UserId != ref.UserId || recovery.Livemode != input.Livemode ||
			!strings.EqualFold(recovery.Currency, input.Currency) {
			return fmt.Errorf("%w: recovery snapshot mismatch", ErrStripeAdjustmentMismatch)
		}

		var adjustment StripePaymentAdjustment
		query := tx.Where("object_type = ? AND object_id = ? AND livemode = ?", input.ObjectType, input.ObjectId, input.Livemode).
			Limit(1).Find(&adjustment)
		if query.Error != nil {
			return query.Error
		}
		now := common.GetTimestamp()
		if query.RowsAffected > 0 {
			if adjustment.PaymentReferenceId != ref.Id || adjustment.TargetKind != ref.TargetKind ||
				adjustment.TargetId != ref.TargetId || adjustment.AmountMinor != input.AmountMinor ||
				!strings.EqualFold(adjustment.Currency, input.Currency) {
				return fmt.Errorf("%w: adjustment object changed", ErrStripeAdjustmentMismatch)
			}
			if input.EventPriority < adjustment.EventPriority ||
				(input.EventPriority == adjustment.EventPriority && input.EventCreated <= adjustment.EventCreated) {
				result.RecoveredQuota = recovery.RecoveredQuota
				result.OutstandingQuota = recovery.OutstandingQuota
				return nil
			}
			adjustment.EventId = input.EventId
			adjustment.Status = input.Status
			adjustment.Active = input.Active
			adjustment.EventCreated = input.EventCreated
			adjustment.EventPriority = input.EventPriority
			adjustment.PaymentIntentId = input.PaymentIntentId
			adjustment.ChargeId = input.ChargeId
			adjustment.UpdatedAt = now
			if err := tx.Save(&adjustment).Error; err != nil {
				return err
			}
		} else {
			adjustment = StripePaymentAdjustment{
				ObjectType: input.ObjectType, ObjectId: input.ObjectId, Livemode: input.Livemode,
				PaymentReferenceId: ref.Id, TargetKind: ref.TargetKind, TargetId: ref.TargetId,
				EventId: input.EventId, PaymentIntentId: input.PaymentIntentId, ChargeId: input.ChargeId,
				AmountMinor: input.AmountMinor, Currency: input.Currency, Status: input.Status,
				Active: input.Active, EventCreated: input.EventCreated, EventPriority: input.EventPriority,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&adjustment).Error; err != nil {
				return err
			}
		}

		var paymentRefs []StripePaymentReference
		if err := tx.Where("target_kind = ? AND target_id = ?", ref.TargetKind, ref.TargetId).
			Find(&paymentRefs).Error; err != nil {
			return err
		}
		paymentAmounts := make(map[int]int64, len(paymentRefs))
		for _, paymentRef := range paymentRefs {
			paymentAmounts[paymentRef.Id] = paymentRef.AmountMinor
		}
		var activeAdjustments []StripePaymentAdjustment
		if err := tx.Where(
			"target_kind = ? AND target_id = ? AND active = ?",
			ref.TargetKind, ref.TargetId, true,
		).Find(&activeAdjustments).Error; err != nil {
			return err
		}
		type paymentLoss struct {
			refund  int64
			dispute int64
		}
		losses := make(map[int]paymentLoss, len(paymentRefs))
		for _, activeAdjustment := range activeAdjustments {
			paymentAmount, ok := paymentAmounts[activeAdjustment.PaymentReferenceId]
			if !ok || paymentAmount <= 0 {
				return fmt.Errorf("%w: adjustment references an unknown payment", ErrStripeAdjustmentMismatch)
			}
			loss := losses[activeAdjustment.PaymentReferenceId]
			switch activeAdjustment.ObjectType {
			case StripeAdjustmentRefund:
				if loss.refund >= paymentAmount-activeAdjustment.AmountMinor {
					loss.refund = paymentAmount
				} else {
					loss.refund += activeAdjustment.AmountMinor
				}
			case StripeAdjustmentDispute:
				if activeAdjustment.AmountMinor > loss.dispute {
					loss.dispute = activeAdjustment.AmountMinor
				}
			default:
				return fmt.Errorf("%w: unknown adjustment type", ErrStripeAdjustmentMismatch)
			}
			losses[activeAdjustment.PaymentReferenceId] = loss
		}
		activeLoss := int64(0)
		for _, paymentRef := range paymentRefs {
			loss := losses[paymentRef.Id]
			paymentActiveLoss := loss.refund
			if loss.dispute > paymentActiveLoss {
				paymentActiveLoss = loss.dispute
			}
			if activeLoss >= recovery.OriginalAmountMinor-paymentActiveLoss {
				activeLoss = recovery.OriginalAmountMinor
				break
			}
			activeLoss += paymentActiveLoss
		}

		if recovery.TargetKind == StripePaymentTargetTopUp {
			desiredDecimal := decimal.NewFromInt(recovery.OriginalQuota).
				Mul(decimal.NewFromInt(activeLoss)).
				Div(decimal.NewFromInt(recovery.OriginalAmountMinor)).Floor()
			desired, clamp := common.QuotaFromDecimalChecked(desiredDecimal)
			if clamp != nil || desired < 0 {
				return fmt.Errorf("%w: recovered quota is out of range", ErrStripeAdjustmentMismatch)
			}
			if err := applyTopUpRecoveryTx(tx, &user, &recovery, int64(desired)); err != nil {
				return err
			}
		} else if recovery.TargetKind == StripePaymentTargetSubscription {
			changed, err := applySubscriptionRecoveryTx(tx, &user, &recovery, activeLoss)
			if err != nil {
				return err
			}
			groupChanged = changed
		} else {
			return fmt.Errorf("%w: unknown recovery target", ErrStripeAdjustmentMismatch)
		}
		recovery.UpdatedAt = now
		if err := tx.Save(&recovery).Error; err != nil {
			return err
		}
		if user.Quota > 0 && user.BillingDebt > 0 {
			debtPayment := int64(user.Quota)
			if debtPayment > user.BillingDebt {
				debtPayment = user.BillingDebt
			}
			paid, err := applyStripeBillingDebtPaymentTx(tx, user.Id, debtPayment)
			if err != nil {
				return err
			}
			user.Quota -= int(paid)
			user.BillingDebt -= paid
		}
		if user.BillingDebt < 0 {
			return fmt.Errorf("%w: user billing debt became negative", ErrStripeAdjustmentMismatch)
		}
		if err := tx.Model(&user).Updates(map[string]interface{}{
			"quota": user.Quota, "billing_debt": user.BillingDebt,
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", recovery.Id).First(&recovery).Error; err != nil {
			return err
		}
		result.RecoveredQuota = recovery.RecoveredQuota
		result.OutstandingQuota = recovery.OutstandingQuota
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := InvalidateUserCache(result.UserId); err != nil {
		common.SysError("failed to invalidate user cache after Stripe adjustment: " + err.Error())
	}
	if groupChanged {
		refreshSubscriptionUserGroupCache(result.UserId, "Stripe payment adjustment")
	}
	return result, nil
}

func applyTopUpRecoveryTx(tx *gorm.DB, user *User, recovery *StripePaymentRecovery, desired int64) error {
	if desired < 0 || desired > recovery.OriginalQuota {
		return fmt.Errorf("%w: invalid top-up recovery target", ErrStripeAdjustmentMismatch)
	}
	delta := desired - recovery.RecoveredQuota
	if delta > 0 {
		available := int64(user.Quota)
		if available < 0 {
			available = 0
		}
		applied := delta
		if applied > available {
			applied = available
		}
		user.Quota -= int(applied)
		recovery.WalletRecoveredQuota += applied
		outstanding := delta - applied
		if outstanding > 0 {
			user.BillingDebt += outstanding
			recovery.OutstandingQuota += outstanding
		}
	} else if delta < 0 {
		remaining := -delta
		release := remaining
		if release > recovery.OutstandingQuota {
			release = recovery.OutstandingQuota
		}
		recovery.OutstandingQuota -= release
		user.BillingDebt -= release
		remaining -= release

		release = remaining
		if release > recovery.DebtPaidQuota {
			release = recovery.DebtPaidQuota
		}
		if int64(user.Quota)+release > int64(common.MaxQuota)+user.BillingDebt {
			return errors.New("Stripe adjustment restoration would exceed the quota limit")
		}
		recovery.DebtPaidQuota -= release
		user.Quota += int(release)
		remaining -= release

		if remaining > recovery.WalletRecoveredQuota {
			return fmt.Errorf("%w: recovery accounting is inconsistent", ErrStripeAdjustmentMismatch)
		}
		if int64(user.Quota)+remaining > int64(common.MaxQuota)+user.BillingDebt {
			return errors.New("Stripe adjustment restoration would exceed the quota limit")
		}
		recovery.WalletRecoveredQuota -= remaining
		user.Quota += int(remaining)
	}
	if user.BillingDebt < 0 {
		return fmt.Errorf("%w: user billing debt became negative", ErrStripeAdjustmentMismatch)
	}
	recovery.RecoveredQuota = desired
	return nil
}

func applySubscriptionRecoveryTx(tx *gorm.DB, user *User, recovery *StripePaymentRecovery, activeLoss int64) (bool, error) {
	var settlement StripeSubscriptionSettlement
	if err := tx.Where("id = ?", recovery.TargetId).First(&settlement).Error; err != nil {
		return false, err
	}
	var sub UserSubscription
	if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", settlement.UserSubscriptionId, recovery.UserId).First(&sub).Error; err != nil {
		return false, err
	}
	groupChanged := false
	desired := int64(0)
	newTotal := recovery.OriginalQuota
	revokeEntitlement := false
	if recovery.OriginalQuota == 0 {
		revokeEntitlement = activeLoss >= recovery.OriginalAmountMinor
	} else {
		desiredDecimal := decimal.NewFromInt(recovery.OriginalQuota).
			Mul(decimal.NewFromInt(activeLoss)).
			Div(decimal.NewFromInt(recovery.OriginalAmountMinor)).Floor()
		converted, clamp := common.QuotaFromDecimalChecked(desiredDecimal)
		if clamp != nil || converted < 0 {
			return false, fmt.Errorf("%w: subscription recovery is out of range", ErrStripeAdjustmentMismatch)
		}
		desired = int64(converted)
		newTotal = recovery.OriginalQuota - desired
		revokeEntitlement = newTotal == 0 && activeLoss > 0
	}

	debtTarget := int64(0)
	if recovery.OriginalQuota > 0 && sub.AmountUsed > newTotal {
		debtTarget = sub.AmountUsed - newTotal
	}
	currentDebtEffect := recovery.OutstandingQuota + recovery.DebtPaidQuota
	if debtTarget > currentDebtEffect {
		increase := debtTarget - currentDebtEffect
		recovery.OutstandingQuota += increase
		user.BillingDebt += increase
	} else if debtTarget < currentDebtEffect {
		remaining := currentDebtEffect - debtTarget
		release := remaining
		if release > recovery.OutstandingQuota {
			release = recovery.OutstandingQuota
		}
		recovery.OutstandingQuota -= release
		user.BillingDebt -= release
		remaining -= release
		if remaining > recovery.DebtPaidQuota {
			return false, fmt.Errorf("%w: subscription debt accounting is inconsistent", ErrStripeAdjustmentMismatch)
		}
		if int64(user.Quota)+remaining > int64(common.MaxQuota)+user.BillingDebt {
			return false, errors.New("Stripe adjustment restoration would exceed the quota limit")
		}
		recovery.DebtPaidQuota -= remaining
		user.Quota += int(remaining)
	}
	if user.BillingDebt < 0 {
		return false, fmt.Errorf("%w: user billing debt became negative", ErrStripeAdjustmentMismatch)
	}

	wasRevoked := recovery.EntitlementRevoked
	if recovery.OriginalQuota > 0 {
		sub.AmountTotal = newTotal
	}
	if revokeEntitlement && !wasRevoked {
		sub.Status = "cancelled"
		if target, err := downgradeUserGroupForSubscriptionTx(tx, &sub, common.GetTimestamp()); err != nil {
			return false, err
		} else if target != "" {
			groupChanged = true
		}
	} else if !revokeEntitlement && wasRevoked && sub.EndTime > common.GetTimestamp() {
		sub.Status = "active"
		upgradeGroup := strings.TrimSpace(sub.UpgradeGroup)
		if upgradeGroup != "" {
			var currentGroup string
			if err := tx.Model(&User{}).Where("id = ?", sub.UserId).Select(commonGroupCol).Find(&currentGroup).Error; err != nil {
				return false, err
			}
			if currentGroup != upgradeGroup {
				if err := tx.Model(&User{}).Where("id = ?", sub.UserId).Update("group", upgradeGroup).Error; err != nil {
					return false, err
				}
				groupChanged = true
			}
		}
	}
	if err := tx.Save(&sub).Error; err != nil {
		return false, err
	}
	recovery.RecoveredQuota = desired
	recovery.EntitlementRevoked = revokeEntitlement
	return groupChanged, nil
}

func applyStripeBillingDebtPaymentTx(tx *gorm.DB, userId int, amount int64) (int64, error) {
	if tx == nil || userId <= 0 || amount <= 0 {
		return 0, nil
	}
	var recoveries []StripePaymentRecovery
	if err := lockForUpdate(tx).
		Where("user_id = ? AND outstanding_quota > 0", userId).
		Order("created_at asc, id asc").
		Find(&recoveries).Error; err != nil {
		return 0, err
	}
	remaining := amount
	for i := range recoveries {
		if remaining == 0 {
			break
		}
		payment := recoveries[i].OutstandingQuota
		if payment > remaining {
			payment = remaining
		}
		recoveries[i].OutstandingQuota -= payment
		recoveries[i].DebtPaidQuota += payment
		recoveries[i].UpdatedAt = common.GetTimestamp()
		if err := tx.Save(&recoveries[i]).Error; err != nil {
			return 0, err
		}
		remaining -= payment
	}
	paid := amount - remaining
	if paid != amount {
		return 0, fmt.Errorf("%w: billing debt has no matching recovery records", ErrStripeAdjustmentMismatch)
	}
	return paid, nil
}
