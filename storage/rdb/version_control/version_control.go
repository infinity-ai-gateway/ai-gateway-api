// Copyright (c) 2021 The BFE Authors.
//
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

package version_control

import (
	"context"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iversion_control"
	"github.com/rainway-ai-gateway/ai-gateway-api/storage/rdb/internal/dao"
)

const (
	versionLayout       = "20060102150405"
	maxDuplicateRetries = 3
)

var _ iversion_control.VersionControlStorager = &VersionControlStorager{}

type VersionControlStorager struct {
	dbCtxFactory lib.DBContextFactory
}

func NewVersionControllerStorage(dbCtxFactory lib.DBContextFactory) *VersionControlStorager {
	return &VersionControlStorager{
		dbCtxFactory: dbCtxFactory,
	}
}

func (vcs *VersionControlStorager) UpsertConfigLastExportedVersion(ctx context.Context, css *iversion_control.ExportData) (string, error) {
	dbCtx, err := vcs.dbCtxFactory(ctx)
	if err != nil {
		return "", err
	}

	latest, err := latestConfigVersion(dbCtx, css.Topic)
	if err != nil {
		return "", err
	}

	if latest != nil && latest.DataSign == css.DataSignWithoutVersion {
		return latest.Version, nil
	}

	version := iversion_control.Version(time.Now())
	if latest != nil {
		version = bumpVersion(latest.Version, version)
	}

	for i := 0; i < maxDuplicateRetries; i++ {
		_, err = dao.TConfigVersionCreate(dbCtx, &dao.TConfigVersionParam{
			Name:     &css.Topic,
			DataSign: &css.DataSignWithoutVersion,
			Version:  &version,
		})
		if err == nil {
			return version, nil
		}
		if !lib.DuplicateEntryError(err) {
			return "", err
		}

		// A concurrent export inserted the same version first: re-read the
		// latest row and bump past it, instead of failing the whole export
		// with a duplicate key error (issue #142).
		latest, err = latestConfigVersion(dbCtx, css.Topic)
		if err != nil {
			return "", err
		}
		if latest == nil {
			return "", err
		}
		if latest.DataSign == css.DataSignWithoutVersion {
			return latest.Version, nil
		}
		version = bumpVersion(latest.Version, version)
	}

	return "", err
}

func latestConfigVersion(dbCtx lib.DBContexter, topic string) (*dao.TConfigVersion, error) {
	return dao.TConfigVersionOne(dbCtx, &dao.TConfigVersionParam{
		Name:    &topic,
		OrderBy: lib.PString("version DESC, id DESC"),
	})
}

// bumpVersion returns latest+1s when it is not before candidate, so the
// result is strictly greater than every existing version of the topic even
// when several config changes are exported within the same wall-clock second.
func bumpVersion(latest string, candidate string) string {
	lt, err := time.ParseInLocation(versionLayout, latest, time.Local)
	if err != nil {
		return candidate
	}
	next := iversion_control.Version(lt.Add(time.Second))
	if candidate < next {
		return next
	}
	return candidate
}
