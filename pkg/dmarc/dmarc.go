/*
 * ZDNS Copyright 2016 Regents of the University of Michigan
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy
 * of the License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
 * implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

package dmarc

import (
	"regexp"

	"github.com/zmap/dns"
	"github.com/zmap/zdns/pkg/miekg"
	"github.com/zmap/zdns/pkg/zdns"
)

const dmarcPrefixRegexp = "^[vV][\x09\x20]*=[\x09\x20]*DMARC1[\x09\x20]*;[\x09\x20]*"

// result to be returned by scan of host
type Result struct {
	Dmarc string `json:"dmarc,omitempty" groups:"short,normal,long,trace"`
}

// Per Connection Lookup ======================================================
type Lookup struct {
	Factory *RoutineLookupFactory
	miekg.Lookup
}

func (s *Lookup) DoLookup(name string, nameServer string) (interface{}, zdns.Trace, zdns.Status, error) {
	innerRes, trace, status, err := s.DoMiekgLookup(miekg.Question{Name: name, Type: s.DNSType, Class: s.DNSClass}, nameServer)
	resString, resStatus, err := s.CheckTxtRecords(innerRes, status, err)
	res := Result{Dmarc: resString}
	return res, trace, resStatus, err
}

// Per GoRoutine Factory ======================================================
type RoutineLookupFactory struct {
	miekg.RoutineLookupFactory
	Factory *GlobalLookupFactory
}

func (rlf *RoutineLookupFactory) MakeLookup() (zdns.Lookup, error) {
	lookup := Lookup{Factory: rlf}
	nameServer := rlf.Factory.RandomNameServer()
	lookup.Initialize(nameServer, dns.TypeTXT, dns.ClassINET, &rlf.RoutineLookupFactory)
	return &lookup, nil
}

func (rlf *RoutineLookupFactory) InitPrefixRegexp() {
	rlf.PrefixRegexp = regexp.MustCompile(dmarcPrefixRegexp)
}

// Global Factory =============================================================
type GlobalLookupFactory struct {
	miekg.GlobalLookupFactory
}

func (glf *GlobalLookupFactory) MakeRoutineFactory(threadID int) (zdns.RoutineLookupFactory, error) {
	rlf := new(RoutineLookupFactory)
	rlf.RoutineLookupFactory.Factory = &glf.GlobalLookupFactory
	rlf.InitPrefixRegexp()
	rlf.Initialize(glf.GlobalConf)
	rlf.Factory = glf
	rlf.ThreadID = threadID
	return rlf, nil
}

// Global Registration ========================================================
func init() {
	s := new(GlobalLookupFactory)
	zdns.RegisterLookup("DMARC", s)
}
