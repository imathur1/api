package controller

import (
	"encoding/json"
	"net/http"

	"github.com/HackIllinois/api/common/authtoken"
	"github.com/HackIllinois/api/common/errors"
	"github.com/HackIllinois/api/common/metrics"
	"github.com/HackIllinois/api/common/utils"
	"github.com/HackIllinois/api/services/event/models"
	"github.com/HackIllinois/api/services/event/service"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func SetupController(route *mux.Route) {
	router := route.Subrouter()

	router.Handle("/internal/metrics/", promhttp.Handler()).Methods("GET")

	metrics.RegisterHandler("/favorite/", GetEventFavorites, "GET", router)
	metrics.RegisterHandler("/favorite/", AddEventFavorite, "POST", router)
	metrics.RegisterHandler("/favorite/", RemoveEventFavorite, "DELETE", router)

	metrics.RegisterHandler("/filter/", GetFilteredEvents, "GET", router)
	metrics.RegisterHandler("/{id}/", GetEvent, "GET", router)
	metrics.RegisterHandler("/{id}/", DeleteEvent, "DELETE", router)
	metrics.RegisterHandler("/", CreateEvent, "POST", router)
	metrics.RegisterHandler("/", UpdateEvent, "PUT", router)
	metrics.RegisterHandler("/", GetAllEvents, "GET", router)
	metrics.RegisterHandler("/code/{id}/", GetEventCode, "GET", router)
	metrics.RegisterHandler("/code/{id}/", UpdateEventCode, "PUT", router)

	metrics.RegisterHandler("/staff/checkin/", StaffCheckin, "POST", router)
	metrics.RegisterHandler("/checkin/", Checkin, "POST", router)

	metrics.RegisterHandler("/track/", MarkUserAsAttendingEvent, "POST", router)
	metrics.RegisterHandler("/track/event/{id}/", GetEventTrackingInfo, "GET", router)
	metrics.RegisterHandler("/track/user/{id}/", GetUserTrackingInfo, "GET", router)

	metrics.RegisterHandler("/internal/stats/", GetStats, "GET", router)

	// My changes
	metrics.RegisterHandler("/building/{tag}/", GetEventsInBuilding, "GET", router)
}


/*
	My changes
*/
func GetEventsInBuilding(w http.ResponseWriter, r *http.Request) {
	tag := mux.Vars(r)["tag"]

	if tag == "" {
		errors.WriteError(
			w,
			r,
			errors.MalformedRequestError("Must provide building tag in request url.", "Must provide building tag in request url."),
		)
		return
	}

	code, err := service.GetEventsInBuilding[models.EventDB](tag)
	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Failed to receive event code information from database"))
		return
	}

	json.NewEncoder(w).Encode(code)
}

/*
	Endpoint to get the event with the specified id
*/
func GetEvent(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	is_staff := authtoken.IsRequestFromStaffOrHigher(r)

	var event interface{}
	var err error

	if is_staff {
		event, err = service.GetEvent[models.EventDB](id)
	} else {
		event, err = service.GetEvent[models.EventPublic](id)
	}

	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Could not fetch the event details."))
		return
	}

	json.NewEncoder(w).Encode(event)
}

/*
	Endpoint to delete an event with the specified id.
	It removes the event from the event trackers, and every user's tracker.
	On successful deletion, it returns the event that was deleted.
*/
func DeleteEvent(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	event, err := service.DeleteEvent(id)
	if err != nil {
		errors.WriteError(
			w,
			r,
			errors.InternalError(
				err.Error(),
				"Could not delete either the event, event trackers, or user trackers, or an intermediary subroutine failed.",
			),
		)
		return
	}

	json.NewEncoder(w).Encode(event)
}

/*
	Endpoint to get all events
*/
func GetAllEvents(w http.ResponseWriter, r *http.Request) {
	is_staff := authtoken.IsRequestFromStaffOrHigher(r)

	var event_list interface{}
	var err error

	if is_staff {
		event_list, err = service.GetAllEvents[models.EventDB]()
	} else {
		event_list, err = service.GetAllEvents[models.EventPublic]()
	}

	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Could not get all events."))
		return
	}

	json.NewEncoder(w).Encode(event_list)
}

/*
	Endpoint to get events based on filters
*/
func GetFilteredEvents(w http.ResponseWriter, r *http.Request) {
	parameters := r.URL.Query()
	is_staff := authtoken.IsRequestFromStaffOrHigher(r)

	var event interface{}
	var err error

	if is_staff {
		event, err = service.GetFilteredEvents[models.EventDB](parameters)
	} else {
		event, err = service.GetFilteredEvents[models.EventPublic](parameters)
	}

	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Could not fetch filtered list of events."))
		return
	}

	json.NewEncoder(w).Encode(event)
}

/*
	Endpoint to create an event
*/
func CreateEvent(w http.ResponseWriter, r *http.Request) {
	var event models.EventDB
	json.NewDecoder(r.Body).Decode(&event)

	event.ID = utils.GenerateUniqueID()
	code := utils.GenerateUniqueCode()

	err := service.CreateEvent(event.ID, code, event)
	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Could not create new event."))
		return
	}

	updated_event, err := service.GetEvent[models.EventDB](event.ID)
	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Could not get updated event."))
		return
	}

	json.NewEncoder(w).Encode(updated_event)
}

/*
	Endpoint to update an event
*/
func UpdateEvent(w http.ResponseWriter, r *http.Request) {
	var event models.EventDB
	json.NewDecoder(r.Body).Decode(&event)

	err := service.UpdateEvent(event.ID, event)
	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Could not update the event."))
		return
	}

	updated_event, err := service.GetEvent[models.EventDB](event.ID)
	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Could not get updated event details."))
		return
	}

	json.NewEncoder(w).Encode(updated_event)
}

/*
	Endpoint to get the code associated with an event (or nil)
*/
func GetEventCode(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	if id == "" {
		errors.WriteError(
			w,
			r,
			errors.MalformedRequestError("Must provide event id in request url.", "Must provide event id in request url."),
		)
		return
	}

	code, err := service.GetEventCode(id)
	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Failed to receive event code information from database"))
		return
	}

	json.NewEncoder(w).Encode(code)
}

/*
	Endpoint to update an event code and end time
*/
func UpdateEventCode(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	if id == "" {
		errors.WriteError(
			w,
			r,
			errors.MalformedRequestError("Must provide event id in request url.", "Must provide event id in request url."),
		)
		return
	}

	var eventCode models.EventCode
	json.NewDecoder(r.Body).Decode(&eventCode)

	eventCode.ID = id

	err := service.UpdateEventCode(id, eventCode)
	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Could not update the code and timestamp of the event."))
		return
	}

	updated_event, err := service.GetEventCode(id)
	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Could not get updated event code and timestamp details."))
		return
	}

	json.NewEncoder(w).Encode(updated_event)
}

/*
	Endpoint to allow a staff member to check in an attendee on their behalf using a userToken and eventId
*/
func StaffCheckin(w http.ResponseWriter, r *http.Request) {
	var staff_checkin_request models.StaffCheckinRequest
	json.NewDecoder(r.Body).Decode(&staff_checkin_request)

	// We've gotten the user id and event id, now we need to checkin
	response, err := service.CheckinUserTokenToEvent(staff_checkin_request.UserToken, staff_checkin_request.EventID)
	// If there was an error, write it out
	if err != nil {
		errors.WriteError(w, r, errors.UnknownError(err.Error(), err.Error()))
	}

	// Otherwise, we can just write the response
	json.NewEncoder(w).Encode(response)
}

/*
	Endpoint to checkin to a non-staff event using a code
*/
func Checkin(w http.ResponseWriter, r *http.Request) {
	user_id := r.Header.Get("HackIllinois-Identity")

	if user_id == "" {
		errors.WriteError(w, r, errors.MalformedRequestError("Must provide id in request.", "Must provide id in request."))
		return
	}

	var checkin_request models.CheckinRequest
	json.NewDecoder(r.Body).Decode(&checkin_request)

	// We've got the user_id and code for the vent
	response, err := service.CheckinUserByCode(user_id, checkin_request.Code)
	// If there was an error, write it out
	if err != nil {
		errors.WriteError(w, r, errors.UnknownError(err.Error(), err.Error()))
	}

	// Otherwise, we can just write the response
	json.NewEncoder(w).Encode(response)
}

/*
	Endpoint to get tracking info by event
*/
func GetEventTrackingInfo(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	tracker, err := service.GetEventTracker(id)
	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Could not get event tracker."))
		return
	}

	json.NewEncoder(w).Encode(tracker)
}

/*
	Endpoint to get tracking info by user
*/
func GetUserTrackingInfo(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	tracker, err := service.GetUserTracker(id)
	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Could not get user tracker."))
		return
	}

	json.NewEncoder(w).Encode(tracker)
}

/*
	Mark a user as attending an event
*/
func MarkUserAsAttendingEvent(w http.ResponseWriter, r *http.Request) {
	var tracking_info models.TrackingInfo
	json.NewDecoder(r.Body).Decode(&tracking_info)

	is_checkedin, err := service.IsUserCheckedIn(tracking_info.UserID)
	if err != nil {
		errors.WriteError(w, r, errors.InternalError(err.Error(), "Could not determine check-in status of user."))
		return
	}

	if !is_checkedin {
		errors.WriteError(
			w,
			r,
			errors.AttributeMismatchError(
				"User must be checked-in to attend event.",
				"User must be checked-in to attend event.",
			),
		)
		return
	}

	err = service.MarkUserAsAttendingEvent(tracking_info.EventID, tracking_info.UserID)

	if err != nil {
		if err.Error() == "User has already been marked as attending" {
			errors.WriteError(
				w,
				r,
				errors.AttributeMismatchError("User has already checked in.", "User has already checked in."),
			)
		} else if err.Error() == "People cannot be checked-in for the event at this time." {
			errors.WriteError(w, r, errors.AttributeMismatchError("Event is not open for check-in at this time.", "Event is not open for check-in at this time."))
		} else {
			errors.WriteError(w, r, errors.InternalError(err.Error(), "Could not mark user as attending the event."))
		}
		return
	}

	event_tracker, err := service.GetEventTracker(tracking_info.EventID)
	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Could not get event trackers."))
		return
	}

	user_tracker, err := service.GetUserTracker(tracking_info.UserID)
	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Could not get user trackers."))
		return
	}

	tracking_status := &models.TrackingStatus{
		EventTracker: *event_tracker,
		UserTracker:  *user_tracker,
	}

	json.NewEncoder(w).Encode(tracking_status)
}

/*
	Endpoint to get the current user's event favorites
*/
func GetEventFavorites(w http.ResponseWriter, r *http.Request) {
	id := r.Header.Get("HackIllinois-Identity")

	favorites, err := service.GetEventFavorites(id)
	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Could not get user's event favorites."))
		return
	}

	json.NewEncoder(w).Encode(favorites)
}

/*
	Endpoint to add an event favorite for the current user
*/
func AddEventFavorite(w http.ResponseWriter, r *http.Request) {
	id := r.Header.Get("HackIllinois-Identity")

	var event_favorite_modification models.EventFavoriteModification
	json.NewDecoder(r.Body).Decode(&event_favorite_modification)

	err := service.AddEventFavorite(id, event_favorite_modification.EventID)
	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Could not add an event favorite for the current user."))
		return
	}

	favorites, err := service.GetEventFavorites(id)
	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Could not get updated user event favorites."))
		return
	}

	json.NewEncoder(w).Encode(favorites)
}

/*
	Endpoint to remove an event favorite for the current user
*/
func RemoveEventFavorite(w http.ResponseWriter, r *http.Request) {
	id := r.Header.Get("HackIllinois-Identity")

	var event_favorite_modification models.EventFavoriteModification
	json.NewDecoder(r.Body).Decode(&event_favorite_modification)

	err := service.RemoveEventFavorite(id, event_favorite_modification.EventID)
	if err != nil {
		errors.WriteError(w, r, errors.DatabaseError(err.Error(), "Could not remove an event favorite for the current user."))
		return
	}

	favorites, err := service.GetEventFavorites(id)
	if err != nil {
		errors.WriteError(
			w,
			r,
			errors.DatabaseError(err.Error(), "Could not fetch updated event favorites for the user (post-removal)."),
		)
		return
	}

	json.NewEncoder(w).Encode(favorites)
}

/*
	Endpoint to get event stats
*/
func GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := service.GetStats()
	if err != nil {
		errors.WriteError(w, r, errors.InternalError(err.Error(), "Could not fetch event service statistics."))
		return
	}

	json.NewEncoder(w).Encode(stats)
}
