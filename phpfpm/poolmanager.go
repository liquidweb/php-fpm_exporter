// Copyright © 2018 Enrico Stahn <enrico.stahn@gmail.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package phpfpm provides convenient access to PHP-FPM pool data
package phpfpm

import (
	"context"
	"sync"
	"time"

	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

// PoolManager manages all configured Pools
type PoolManager struct {
	Pools map[string]*Pool `json:"pools"`
	mutex sync.RWMutex
}

// Add will add a pool to the pool manager based on the given URI.
func (pm *PoolManager) Add(uri string) *Pool {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if pm.Pools == nil {
		pm.Pools = make(map[string]*Pool)
	}

	p := &Pool{Address: uri}
	log.Infof("Adding pool %v", uri)
	pm.Pools[uri] = p
	return p
}

func (pm *PoolManager) Remove(uri string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	log.Infof("Removing pool: %v", uri)
	delete(pm.Pools, uri)
}

func (pm *PoolManager) WatchDir(ctx context.Context, dir string) {
	go func() {
		// Make the channel buffered to ensure no event is dropped. Notify will drop
		// an event if the receiver is not able to keep up the sending pace.
		c := make(chan notify.EventInfo, 10)

		// Set up a watchpoint listening on events within current working directory.
		// Dispatch each create and remove events separately to c.
		if err := notify.Watch(dir, c, notify.Create, notify.Remove); err != nil {
			log.Fatal(err)
		}
		defer notify.Stop(c)

		// Block until an event is received.
		for {
			select {
			case ev := <-c:
				log.Debugf("Got event: %#v", ev)
				uri := "unix://" + ev.Path() + ";/status"
				if ev.Event() == notify.Create {
					pm.Add(uri)
				} else if ev.Event() == notify.Remove {
					pm.Remove(uri)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// if event.Op&fsnotify.Create == fsnotify.Create {
// 	pm.Add(uri)
// } else if event.Op&fsnotify.Remove == fsnotify.Remove {
// 	pm.Remove(uri)
// }

// Update will run the pool.Update() method concurrently on all Pools.
func (pm *PoolManager) Update() (err error) {
	pm.mutex.RLock()
	wg := &sync.WaitGroup{}

	started := time.Now()

	for _, pool := range pm.Pools {
		wg.Add(1)
		go func(p *Pool) {
			defer wg.Done()
			p.Update()
		}(pool)
	}
	pm.mutex.RUnlock()

	wg.Wait()

	ended := time.Now()

	log.Debugf("Updated %v pool(s) in %v", len(pm.Pools), ended.Sub(started))

	return nil
}
