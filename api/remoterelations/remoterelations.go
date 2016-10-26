// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package remoterelations

import (
	"github.com/juju/errors"
	"gopkg.in/juju/names.v2"

	"github.com/juju/juju/api/base"
	apiwatcher "github.com/juju/juju/api/watcher"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/watcher"
)

const remoteRelationsFacade = "RemoteRelations"

// State provides access to a remoterelations's view of the state.
type State struct {
	facade base.FacadeCaller
}

// NewState creates a new client-side RemoteRelations facade.
func NewState(caller base.APICaller) *State {
	facadeCaller := base.NewFacadeCaller(caller, remoteRelationsFacade)
	return &State{facadeCaller}
}

// WatchRemoteApplications returns a strings watcher that notifies of the addition,
// removal, and lifecycle changes of remote applications in the model.
func (st *State) WatchRemoteApplications() (watcher.StringsWatcher, error) {
	var result params.StringsWatchResult
	err := st.facade.FacadeCall("WatchRemoteApplications", nil, &result)
	if err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, result.Error
	}
	w := apiwatcher.NewStringsWatcher(st.facade.RawAPICaller(), result)
	return w, nil
}

// WatchRemoteApplication returns application relations watchers that delivers
// changes according to the addition, removal, and lifecycle changes of
// relations that the specified remote application is involved in; and also
// according to the entering, departing, and change of unit settings in
// those relations.
func (st *State) WatchRemoteApplication(service string) (watcher.ApplicationRelationsWatcher, error) {
	if !names.IsValidApplication(service) {
		return nil, errors.NotValidf("application name %q", service)
	}
	serviceTag := names.NewApplicationTag(service)
	args := params.Entities{
		Entities: []params.Entity{{Tag: serviceTag.String()}},
	}

	var results params.ApplicationRelationsWatchResults
	err := st.facade.FacadeCall("WatchRemoteApplication", args, &results)
	if err != nil {
		return nil, err
	}
	if len(results.Results) != 1 {
		return nil, errors.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		return nil, result.Error
	}
	w := apiwatcher.NewApplicationRelationsWatcher(st.facade.RawAPICaller(), result)
	return w, nil
}