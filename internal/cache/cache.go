package cache

import (
	"sort"
	"sync"
	"time"

	ent "github.com/Vladroon22/CVmaker/internal/entity"
)

type Cache struct {
	cache map[int][]*ent.CV
	mtx   sync.RWMutex
}

func InitCache() *Cache {
	chc := &Cache{
		cache: make(map[int][]*ent.CV),
		mtx:   sync.RWMutex{},
	}

	go chc.cleanRecords()

	return chc
}

func (c *Cache) Set(id int, cv *ent.CV) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.cache == nil {
		c.cache = make(map[int][]*ent.CV)
	}

	cv.Exp = time.Now().Add(20 * time.Minute)

	c.cache[id] = append(c.cache[id], cv)
}

func (c *Cache) GetLen(id int) int {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	return len(c.cache[id])
}

func (c *Cache) Get(prof string, id int) (*ent.CV, bool) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	return binSearch(c.cache[id], id, prof)
}

func (c *Cache) FromToSliceByID(id int) []ent.CV {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	cached := c.cache[id]

	cvs := make([]ent.CV, 0, len(cached))

	for _, cv := range cached {
		cvs = append(cvs, *cv)
	}

	return cvs
}

func (c *Cache) Delete(prof string, id int) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	for _, cv := range c.cache[id] {
		if cv.Profession == prof {
			delete(c.cache, id)
		}
	}
}

func (c *Cache) cleanRecords() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanExpired()
	}
}

func (c *Cache) cleanExpired() {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	now := time.Now()

	for key, cvs := range c.cache {
		j := 0
		for _, cv := range cvs {
			if cv.Exp.After(now) {
				cvs[j] = cv
				j++
			}
		}

		if j == 0 {
			delete(c.cache, key)
		} else {
			c.cache[key] = cvs[:j]
		}
	}
}

func binSearch(cvs []*ent.CV, goal int, prof string) (*ent.CV, bool) {
	if len(cvs) == 0 {
		return nil, false
	}

	sort.Slice(cvs, func(i, j int) bool { return cvs[i].ID < cvs[j].ID })

	beg := 0
	end := len(cvs) - 1

	for beg <= end {
		mid := beg + (end-beg)/2
		if cvs[mid].ID == goal && cvs[mid].Profession == prof {
			return cvs[mid], true
		} else if cvs[mid].ID < goal {
			beg = mid + 1
		} else {
			end = mid - 1
		}
	}
	return nil, false
}
