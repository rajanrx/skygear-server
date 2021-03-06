// Copyright 2015-present Oursky Ltd.
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

package hooktest

import (
	"context"

	"github.com/skygeario/skygear-server/pkg/server/skydb"
	"github.com/skygeario/skygear-server/pkg/server/skyerr"
)

type StackingHook struct {
	Context         []context.Context
	Records         []*skydb.Record
	OriginalRecords []*skydb.Record
}

func (p *StackingHook) Func(ctx context.Context, record *skydb.Record, originalRecord *skydb.Record) skyerr.Error {
	p.Context = append(p.Context, ctx)
	p.Records = append(p.Records, record)
	p.OriginalRecords = append(p.OriginalRecords, originalRecord)
	return nil
}
