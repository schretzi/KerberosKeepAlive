package krb

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jcmturner/gokrb5/v8/credentials"
	"github.com/jcmturner/gokrb5/v8/iana/nametype"
	"github.com/jcmturner/gokrb5/v8/types"
)

// TicketStatus summarizes the state of a single profile's credential cache.
type TicketStatus struct {
	Exists    bool
	Principal string
	Realm     string
	StartTime time.Time
	EndTime   time.Time
	RenewTill time.Time
	Remaining time.Duration
	Expired   bool
}

// ReadStatus loads the credential cache at ccachePath (if any) and reports
// its krbtgt ticket's validity. A missing file is not an error: it just
// yields TicketStatus{Exists: false}.
func ReadStatus(ccachePath string) (TicketStatus, error) {
	if _, err := os.Stat(ccachePath); errors.Is(err, os.ErrNotExist) {
		return TicketStatus{Exists: false}, nil
	}

	cc, err := credentials.LoadCCache(ccachePath)
	if err != nil {
		return TicketStatus{}, fmt.Errorf("loading ccache %s: %w", ccachePath, err)
	}

	realm := cc.GetClientRealm()
	entry, ok := cc.GetEntry(types.PrincipalName{
		NameType:   nametype.KRB_NT_SRV_INST,
		NameString: []string{"krbtgt", realm},
	})
	if !ok {
		return TicketStatus{Exists: true, Realm: realm}, fmt.Errorf("no krbtgt entry found in ccache %s", ccachePath)
	}

	remaining := time.Until(entry.EndTime)
	return TicketStatus{
		Exists:    true,
		Principal: cc.GetClientPrincipalName().PrincipalNameString() + "@" + realm,
		Realm:     realm,
		StartTime: entry.StartTime,
		EndTime:   entry.EndTime,
		RenewTill: entry.RenewTill,
		Remaining: remaining,
		Expired:   remaining <= 0,
	}, nil
}
