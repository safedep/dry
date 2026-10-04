// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package semantic is a copy of the version parsers of
// github.com/google/osv-scalibr/semantic v0.5.3 for the ecosystems that dry
// orders. dry copies the code and does not import the module, because the
// module go.mod raises the dependencies of every dry user. parse.go of the
// original holds this error. The dispatch by ecosystem lives in api/pb.
// utilities.go drops isASCIILetter, which only the parsers that dry does not
// copy use. Make the same two changes to move to a newer release.
package semantic

import "errors"

// ErrInvalidVersion is the error of a version that a parser rejects.
var ErrInvalidVersion = errors.New("invalid version")
