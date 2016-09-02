package database

import (
	"bytes"
	"encoding/gob"
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/boltdb/bolt"

	"github.com/pylls/datatrack/ephemeral"
	"github.com/pylls/datatrack/model"
)

// AddDisclosure adds a disclosure to the database.
func AddDisclosure(d model.Disclosure) (err error) {
	return DB.Batch(func(tx *bolt.Tx) error {
		db, err := tx.CreateBucketIfNotExists([]byte("disclosure"))
		if err != nil {
			return err
		}
		dbmap, err := tx.CreateBucketIfNotExists([]byte("disclosure map"))
		if err != nil {
			return err
		}
		encoded := new(bytes.Buffer)
		enc := gob.NewEncoder(encoded)
		err = enc.Encode(d)
		if err != nil {
			return err
		}
		err = db.Put([]byte(d.ID), ephemeral.Encrypt(encoded.Bytes()))
		if err != nil {
			return err
		}

		// update sender->id map
		err = appendValueInMap(d.Sender, d.ID, "sender2id", dbmap)
		if err != nil {
			return err
		}
		// update recipient->id map
		err = appendValueInMap(d.Recipient, d.ID, "recipient2id", dbmap)
		if err != nil {
			return err
		}
		// update timestamp-> id map
		err = appendValueInMap(d.Timestamp, d.ID, "timestamp2id", dbmap)
		if err != nil {
			return err
		}

		return nil
	})
}

type sortDisclosures []model.Disclosure

func (a sortDisclosures) Len() int      { return len(a) }
func (a sortDisclosures) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a sortDisclosures) Less(i, j int) bool {
	return strings.Compare(a[i].ID, a[j].ID) == -1
}

// AddDisclosures adds many disclosures at once.
func AddDisclosures(ds []model.Disclosure, wg *sync.WaitGroup, errChan chan error) {
	defer wg.Done()
	sort.Sort(sortDisclosures(ds))

	err := DB.Batch(func(tx *bolt.Tx) error {
		db, err := tx.CreateBucketIfNotExists([]byte("disclosure"))
		if err != nil {
			return err
		}
		dbmap, err := tx.CreateBucketIfNotExists([]byte("disclosure map"))
		if err != nil {
			return err
		}

		// read maps
		sender2id, err := getMap("sender2id", dbmap)
		if err != nil {
			return err
		}
		recipient2id, err := getMap("recipient2id", dbmap)
		if err != nil {
			return err
		}
		timestamp2id, err := getMap("timestamp2id", dbmap)
		if err != nil {
			return err
		}

		// add each disclosure
		for _, d := range ds {
			encoded := new(bytes.Buffer)
			enc := gob.NewEncoder(encoded)
			err = enc.Encode(d)
			if err != nil {
				return err
			}
			err = db.Put([]byte(d.ID), ephemeral.Encrypt(encoded.Bytes()))
			if err != nil {
				return err
			}

			// update sender->id map
			data, exists := sender2id[d.Sender]
			if !exists {
				data = make([]string, 0, 1)
			}
			sender2id[d.Sender] = append(data, d.ID)

			// update recipient->id map
			data, exists = recipient2id[d.Recipient]
			if !exists {
				data = make([]string, 0, 1)
			}
			recipient2id[d.Recipient] = append(data, d.ID)

			// update timestamp-> id map
			data, exists = timestamp2id[d.Timestamp]
			if !exists {
				data = make([]string, 0, 1)
			}
			timestamp2id[d.Timestamp] = append(data, d.ID)
		}

		// write maps
		err = writeMap("sender2id", sender2id, dbmap)
		if err != nil {
			return err
		}
		err = writeMap("recipient2id", recipient2id, dbmap)
		if err != nil {
			return err
		}
		err = writeMap("timestamp2id", timestamp2id, dbmap)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		errChan <- err
	}
}

// GetDisclosure returns the disclosure with the provided id.
func GetDisclosure(id string) (d *model.Disclosure, err error) {
	err = DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("disclosure"))
		if b == nil {
			return errors.New("no disclosure bucket")
		}
		raw := ephemeral.Decrypt(b.Get([]byte(id)))
		if raw == nil {
			return errors.New("no such disclosure")
		}
		d = new(model.Disclosure)
		encoded := bytes.NewBuffer(raw)
		dec := gob.NewDecoder(encoded)
		err = dec.Decode(d)
		if err != nil {
			return err
		}
		return nil
	})
	return
}

// filter out any disclosures not made by Self.
func filterSelf(IDs []string) (result []string, err error) {
	result = make([]string, 0, len(IDs))
	err = DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("disclosure map"))
		if b == nil {
			return errors.New("no disclosure map bucket")
		}
		themap, err := getMap("sender2id", b)
		if err != nil {
			return err
		}
		list, exists := themap[Self]
		if !exists {
			return nil
		}
		sort.Strings(list)
		for i := 0; i < len(IDs); i++ {
			if inList(list, IDs[i]) {
				result = append(result, IDs[i])
			}
		}
		return nil
	})
	return
}

func inList(list []string, item string) bool {
	for _, val := range list {
		if val == item {
			return true
		}
	}
	return false
}

// GetDisclosureIDs returns all the data disclosure IDs.
func GetDisclosureIDs() (IDs []string, err error) {
	IDs = make([]string, 0)
	err = DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("disclosure"))
		if b == nil {
			return errors.New("no disclosure bucket")
		}
		return b.ForEach(func(k, v []byte) error {
			result := make([]byte, len(k))
			copy(result, k)
			IDs = append(IDs, string(result))
			return nil
		})
	})
	return filterSelf(IDs)
}

// GetDisclosureIDsChrono returns all the data disclosure IDs sorted chronologically.
func GetDisclosureIDsChrono() (IDs []string, err error) {
	IDs = make([]string, 0)
	err = DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("disclosure map"))
		if b == nil {
			return errors.New("no disclosure map bucket")
		}

		var timestamp2id map[string][]string
		raw := ephemeral.Decrypt(b.Get([]byte("timestamp2id")))
		if raw == nil {
			return errors.New("no timestamp2id map")
		}
		encoded := bytes.NewBuffer(raw)
		dec := gob.NewDecoder(encoded)
		err = dec.Decode(&timestamp2id)
		if err != nil {
			return err
		}

		// get timestamps into slice
		timestamps := make([]string, 0, len(timestamp2id))
		for key := range timestamp2id {
			timestamps = append(timestamps, key)
		}
		// sort them
		sort.Strings(timestamps)

		// get all IDs, done
		for i := 0; i < len(timestamps); i++ {
			l, exists := timestamp2id[timestamps[i]]
			if !exists {
				panic("should never happen")
			}
			for _, v := range l {
				result := make([]byte, len(v))
				copy(result, v)
				IDs = append(IDs, string(result))
			}
		}

		return nil
	})
	return filterSelf(IDs)
}

// GetDisclosureIDsToOrg returns all data disclosure IDs to a particular organization.
func GetDisclosureIDsToOrg(org string) (IDs []string, err error) {
	IDs = make([]string, 0)
	err = DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("disclosure map"))
		if b == nil {
			return errors.New("no disclosure map bucket")
		}

		var recipient2id map[string][]string
		raw := ephemeral.Decrypt(b.Get([]byte("recipient2id")))
		if raw == nil {
			return errors.New("no recipient2id map")
		}
		encoded := bytes.NewBuffer(raw)
		dec := gob.NewDecoder(encoded)
		err = dec.Decode(&recipient2id)
		if err != nil {
			return err
		}
		list, exists := recipient2id[org]
		if !exists {
			return errors.New("no such organization")
		}
		for i := 0; i < len(list); i++ {
			result := make([]byte, len(list[i]))
			copy(result, list[i])
			IDs = append(IDs, string(result))
		}

		return nil
	})
	return filterSelf(IDs)
}

// GetExplicitDisclosureIDsToOrg is like GetDisclosureIDsToOrg, but explicit.
func GetExplicitDisclosureIDsToOrg(org string) (IDs []string, err error) {
	return getPlicitlyDisclosureIDsToOrg(org, true)
}

// GetImplictDisclosureIDsToOrg is like GetDisclosureIDsToOrg, but implicit.
func GetImplictDisclosureIDsToOrg(org string) (IDs []string, err error) {
	return getPlicitlyDisclosureIDsToOrg(org, false)
}

func getPlicitlyDisclosureIDsToOrg(org string, explicit bool) (IDs []string, err error) {
	IDs = make([]string, 0)
	err = DB.View(func(tx *bolt.Tx) error {
		d := tx.Bucket([]byte("disclosure"))
		if d == nil {
			return errors.New("no disclosure bucket")
		}
		b := tx.Bucket([]byte("disclosure map"))
		if b == nil {
			return errors.New("no disclosure map bucket")
		}

		var recipient2id map[string][]string
		raw := ephemeral.Decrypt(b.Get([]byte("recipient2id")))
		if raw == nil {
			return errors.New("no recipient2id map")
		}
		encoded := bytes.NewBuffer(raw)
		dec := gob.NewDecoder(encoded)
		err = dec.Decode(&recipient2id)
		if err != nil {
			return err
		}
		list, exists := recipient2id[org]
		if !exists {
			return errors.New("no such organization")
		}
		for i := 0; i < len(list); i++ {
			raw := ephemeral.Decrypt(d.Get([]byte(list[i])))
			if raw == nil {
				return errors.New("no such disclosure")
			}
			disc := new(model.Disclosure)
			encoded := bytes.NewBuffer(raw)
			dec := gob.NewDecoder(encoded)
			err = dec.Decode(disc)
			if err != nil {
				return err
			}
			if explicit && !strings.EqualFold(disc.Sender, disc.Recipient) {
				result := make([]byte, len(list[i]))
				copy(result, list[i])
				IDs = append(IDs, string(result))
			} else if !explicit && strings.EqualFold(disc.Sender, disc.Recipient) {
				result := make([]byte, len(list[i]))
				copy(result, list[i])
				IDs = append(IDs, string(result))
			}
		}

		return nil
	})
	return
}

// GetDisclosureIDsToOrgChrono returns all data disclosure IDs to a particular
// organization in chronological order.
func GetDisclosureIDsToOrgChrono(org string) (IDs []string, err error) {
	IDs = make([]string, 0)

	workIDs, err := GetDisclosureIDsToOrg(org)
	if err != nil {
		return
	}
	err = DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("disclosure"))
		if b == nil {
			return errors.New("no disclosure bucket")
		}

		// build time -> id map
		timemap := make(map[string][]string)
		for i := 0; i < len(workIDs); i++ {
			var d model.Disclosure
			raw := ephemeral.Decrypt(b.Get([]byte(workIDs[i])))
			if raw == nil {
				return errors.New("failed to read disclosure")
			}
			encoded := bytes.NewBuffer(raw)
			dec := gob.NewDecoder(encoded)
			err = dec.Decode(&d)
			if err != nil {
				return err
			}
			_, exists := timemap[d.Timestamp]
			if !exists {
				timemap[d.Timestamp] = make([]string, 0)
			}
			timemap[d.Timestamp] = append(timemap[d.Timestamp], d.ID)
		}

		// get only time and sort
		timelist := make([]string, len(timemap))
		for key := range timemap {
			timelist = append(timelist, key)
		}
		sort.Strings(timelist)

		// build id result after sort
		for i := 0; i < len(timelist); i++ {
			for _, v := range timemap[timelist[i]] {
				IDs = append(IDs, v)
			}
		}
		return nil
	})
	return filterSelf(IDs)
}

// GetImplicitDisclosureIDs returns all data disclosures that a recipient of
// the provided disclosure has derived from the provided disclosure.
func GetImplicitDisclosureIDs(id string) (IDs []string, err error) {
	IDs = make([]string, 0)

	err = DB.View(func(tx *bolt.Tx) error {
		origin := tx.Bucket([]byte("downstream origin"))
		if origin == nil {
			return errors.New("no downstream origin bucket")
		}
		disclosure := tx.Bucket([]byte("disclosure"))
		if disclosure == nil {
			return errors.New("no disclosure bucket")
		}

		list, err := getList(id, origin)
		if err != nil {
			return err
		}

		for i := 0; i < len(list); i++ {
			raw := ephemeral.Decrypt(disclosure.Get([]byte(list[i])))
			if raw == nil {
				return errors.New("no such disclosure")
			}
			d := new(model.Disclosure)
			encoded := bytes.NewBuffer(raw)
			dec := gob.NewDecoder(encoded)
			err = dec.Decode(d)
			if err != nil {
				return err
			}
			if strings.EqualFold(d.Sender, d.Recipient) {
				IDs = append(IDs, d.ID)
			}
		}
		return nil

	})
	return
}

// GetImplicitDisclosureIDsChrono is like GetImplicitDisclosureIDs, but also
// sorts the identifiers in chronological order.
func GetImplicitDisclosureIDsChrono(id string) (IDs []string, err error) {
	IDs = make([]string, 0)
	list, err := GetImplicitDisclosureIDs(id)
	if err != nil {
		return
	}

	timestamps := make([]string, 0, len(IDs))
	timestampMap := make(map[string]string)
	for _, id := range list {
		disc, err := GetDisclosure(id)
		if err != nil {
			return nil, err
		}
		timestamps = append(timestamps, disc.Timestamp)
		timestampMap[disc.Timestamp] = disc.ID
	}
	sort.Strings(timestamps)

	for i := 0; i < len(timestamps); i++ {
		IDs = append(IDs, timestampMap[timestamps[i]])
	}

	return
}

// GetDownstreamDisclosureIDs returns all data disclosures (their IDs) that were
// shared downstream with the provided data disclosure as the origin/source.
func GetDownstreamDisclosureIDs(id string) (IDs []string, err error) {
	IDs = make([]string, 0)

	err = DB.View(func(tx *bolt.Tx) error {
		origin := tx.Bucket([]byte("downstream origin"))
		if origin == nil {
			return errors.New("no downstream origin bucket")
		}
		disclosure := tx.Bucket([]byte("disclosure"))
		if disclosure == nil {
			return errors.New("no disclosure bucket")
		}

		list, err := getList(id, origin)
		if err != nil {
			return err
		}

		for i := 0; i < len(list); i++ {
			raw := ephemeral.Decrypt(disclosure.Get([]byte(id)))
			if raw == nil {
				return errors.New("no such disclosure")
			}
			d := new(model.Disclosure)
			encoded := bytes.NewBuffer(raw)
			dec := gob.NewDecoder(encoded)
			err = dec.Decode(d)
			if err != nil {
				return err
			}
			if !strings.EqualFold(d.Sender, d.Recipient) {
				IDs = append(IDs, d.ID)
			}
		}
		return nil

	})
	return
}

// GetDownstreamDisclosureIDsChrono is like GetDownstreamDisclosureIDs, but also
// sorts the identifiers in chronological order.
func GetDownstreamDisclosureIDsChrono(id string) (IDs []string, err error) {
	IDs = make([]string, 0)
	list, err := GetDownstreamDisclosureIDs(id)
	if err != nil {
		return
	}

	timestamps := make([]string, 0, len(IDs))
	timestampMap := make(map[string]string)
	for _, id := range list {
		disc, err := GetDisclosure(id)
		if err != nil {
			return nil, err
		}
		timestamps = append(timestamps, disc.Timestamp)
		timestampMap[disc.Timestamp] = disc.ID
	}
	sort.Strings(timestamps)

	for i := 0; i < len(timestamps); i++ {
		IDs = append(IDs, timestampMap[timestamps[i]])
	}
	return
}
