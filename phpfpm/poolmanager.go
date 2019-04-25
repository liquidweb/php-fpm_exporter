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

	"github.com/fsnotify/fsnotify"
	"github.com/prometheus/common/log"
)

// PoolManager manages all configured Pools
type PoolManager struct {
	Pools map[string]Pool `json:"pools"`
	mutex sync.RWMutex
}

// Add will add a pool to the pool manager based on the given URI.
func (pm *PoolManager) Add(uri string) Pool {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if pm.Pools == nil {
		pm.Pools = make(map[string]Pool)
	}

	p := Pool{Address: uri}
	log.Infof("uri: %s, p: %v+", uri, p)
	pm.Pools[uri] = p
	return p
}

func (pm *PoolManager) Remove(uri string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	delete(pm.Pools, uri)
}

func (pm *PoolManager) WatchDir(ctx context.Context, dir string) {
	go func() {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Fatal(err)
		}
		defer watcher.Close()

		log.Info("Watching for changes in ", dir)
		err = watcher.Add(dir)
		if err != nil {
			log.Fatal(err)
		}

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				uri := "unix://" + dir + event.Name + ";/status"
				if event.Op&fsnotify.Create == fsnotify.Create {
					log.Infof("Adding pool: %v", uri)
					pm.Add(uri)
				} else if event.Op&fsnotify.Remove == fsnotify.Remove {
					log.Infof("Removing pool: %v", uri)
					pm.Remove(uri)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Errorf("dir watcher error: %v", err)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Update will run the pool.Update() method concurrently on all Pools.
func (pm *PoolManager) Update() (err error) {
	pm.mutex.RLock()
	wg := &sync.WaitGroup{}

	started := time.Now()

	for _, pool := range pm.Pools {
		wg.Add(1)
		go func(p Pool) {
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
