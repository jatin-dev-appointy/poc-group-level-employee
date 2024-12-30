package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"go.saastack.io/chaku/driver"
	"go.saastack.io/chaku/errors"
	empPb "go.saastack.io/employee/pb"
	"go.saastack.io/idutil"
	lnkPb "go.saastack.io/link/pb"
	lnk "go.saastack.io/link/store"
	usrPb "go.saastack.io/user/pb"
	"log"
	"net/http"
	"net/url"
)

const (
	DBString               = ""
	GROUP_EMPLOYEE_COMPANY = "GROUP_EMPLOYEE_COMPANY"
)

func initDB() (*sql.DB, error) {
	dbConn, err := sql.Open("postgres", DBString)
	if err != nil {
		return nil, err
	}
	if err = dbConn.Ping(); err != nil {
		return nil, err
	}
	return dbConn, nil
}

func ListGroupEmployees(ctx context.Context, storeImp *StoreImplementation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Handler logic for listing group employees
		vars := mux.Vars(r)
		groupId := vars["groupId"]

		employees, err := storeImp.EmployeeStore.ListEmployees(ctx, empPb.EmployeeAllFields(), empPb.EmployeeFullParentEq{Parent: groupId})
		if err != nil {
			log.Println("error in listing employees: ", err)
			http.Error(w, "error in listing employees", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err = json.NewEncoder(w).Encode(employees); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("error in encoding employees: ", err)
			http.Error(w, "error in encoding employees", http.StatusInternalServerError)
			return
		}
	}
}

func CreateGroupEmployee(ctx context.Context, storeImp *StoreImplementation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Handler logic for creating a group employee
		var employee empPb.Employee
		if err := json.NewDecoder(r.Body).Decode(&employee); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Println("error in decoding employee: ", err)
			http.Error(w, "error in decoding employee", http.StatusBadRequest)
			return
		}

		if employee.Email == "" {
			w.WriteHeader(http.StatusBadRequest)
			log.Println("error in request, email is required")
			http.Error(w, "error in request, email is required", http.StatusBadRequest)
			return
		}

		existEmp, err := storeImp.EmployeeStore.GetEmployee(ctx, []string{}, empPb.EmployeeAnd{
			empPb.EmployeeEmailEq{Email: employee.Email},
			empPb.EmployeeFullParentEq{Parent: employee.Id},
		})
		if err != nil && err != errors.ErrNotFound {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("error in getting employee: ", err)
			http.Error(w, "error in getting employee", http.StatusInternalServerError)
			return
		}

		if existEmp != nil {
			w.WriteHeader(http.StatusConflict)
			log.Println("employee already exists")
			http.Error(w, "employee already exists", http.StatusConflict)
			return
		}

		// donot proceed if user doesn't exist for the given email
		_, err = storeImp.UserStore.GetUserProfile(ctx, []string{}, usrPb.UserProfileEmailEq{Email: employee.Email})
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			log.Println("error in getting user: ", err)
			http.Error(w, "error in getting user", http.StatusNotFound)
			return
		}

		ids, err := storeImp.EmployeeStore.CreateEmployees(ctx, &employee)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("error in creating employee: ", err)
			http.Error(w, "error in creating employee", http.StatusInternalServerError)
			return
		}

		employee.Id = ids[0]
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		if err = json.NewEncoder(w).Encode(employee); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("error in encoding employee: ", err)
			http.Error(w, "error in encoding employee", http.StatusInternalServerError)
			return
		}
	}
}

func DeleteGroupEmployee(ctx context.Context, storeImp *StoreImplementation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Handler logic for deleting a group employee
		vars := mux.Vars(r)
		employeeId := vars["id"]

		err := storeImp.EmployeeStore.DeleteEmployee(ctx, empPb.EmployeeIdEq{Id: idutil.GetId(employeeId)})
		if err != nil {
			log.Println("error in deleting employee: ", err)
			http.Error(w, "error in deleting employee", http.StatusInternalServerError)
			return
		}
	}
}

func ListCompanyLinkedGroupEmployees(ctx context.Context, storeImp *StoreImplementation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Handler logic for listing company's linked group employees
		vars := mux.Vars(r)
		companyId := vars["companyId"]

		// Decode the parameter value
		bytes, err := base64.StdEncoding.DecodeString(companyId)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Println("error in decoding company id: ", err)
			http.Error(w, "error in decoding company id", http.StatusBadRequest)
			return
		}
		companyId = string(bytes)
		companyId, _ = url.QueryUnescape(companyId)

		links, err := storeImp.LinkStore.ListLinks(ctx, &lnkPb.ListLinksRequest{
			Id:       &lnkPb.ListLinksRequest_SecondResourceId{SecondResourceId: companyId},
			LinkType: GROUP_EMPLOYEE_COMPANY,
		})
		if err != nil {
			log.Println("error in listing links: ", err)
			http.Error(w, "error in listing links", http.StatusInternalServerError)
			return
		}

		employeeIds := make([]string, 0, len(links.Links))
		for _, link := range links.Links {
			employeeIds = append(employeeIds, idutil.GetId(link.FirstResourceId))
		}

		employees, err := storeImp.EmployeeStore.ListEmployees(ctx, empPb.EmployeeAllFields(), empPb.EmployeeIdIn{Id: employeeIds})
		if err != nil {
			log.Println("error in listing employees: ", err)
			http.Error(w, "error in listing employees", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err = json.NewEncoder(w).Encode(employees); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("error in encoding employees: ", err)
			http.Error(w, "error in encoding employees", http.StatusInternalServerError)
			return
		}
	}
}

type LinkGroupEmployeeToCompanyRequest struct {
	EmployeeID string `json:"employee_id"`
	CompanyId  string `json:"company_id"`
}

func LinkGroupEmployeeToCompany(ctx context.Context, storeImp *StoreImplementation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Handler logic for linking a group employee to a company
		var request LinkGroupEmployeeToCompanyRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Println("error in decoding request: ", err)
			http.Error(w, "error in decoding request", http.StatusBadRequest)
			return
		}

		err := storeImp.LinkStore.AddLinks(ctx, []*lnkPb.Link{
			{
				FirstResourceId:  request.EmployeeID,
				SecondResourceId: request.CompanyId,
				LinkType:         GROUP_EMPLOYEE_COMPANY,
			},
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println("error in linking employee to company: ", err)
			http.Error(w, "error in linking employee to company", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

type StoreImplementation struct {
	Db            *sql.DB
	EmployeeStore empPb.EmployeeStore
	UserStore     usrPb.UserProfileStore
	LinkStore     lnk.LinkStore
}

func main() {
	ctx := context.Background()

	db, err := initDB()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("connected to db")
	defer func() {
		if err = db.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	dbStore := &StoreImplementation{
		db,
		empPb.NewPostgresEmployeeStore(db, driver.GetUserId),
		usrPb.NewPostgresUserProfileStore(db, driver.GetUserId),
		lnk.NewPostgresLinkStore(db),
	}

	r := mux.NewRouter()
	r.HandleFunc("/employee/{groupId}", ListGroupEmployees(ctx, dbStore)).Methods("GET")
	r.HandleFunc("/employee", CreateGroupEmployee(ctx, dbStore)).Methods("POST")
	r.HandleFunc("/employee/{id}", DeleteGroupEmployee(ctx, dbStore)).Methods("DELETE")
	r.HandleFunc("/employee/company/{companyId}", ListCompanyLinkedGroupEmployees(ctx, dbStore)).Methods("GET")
	r.HandleFunc("/employee/company", LinkGroupEmployeeToCompany(ctx, dbStore)).Methods("POST")

	err = http.ListenAndServe("127.0.0.1:8080", r)
	if err != nil {
		log.Fatal("error starting server: ", err)
	}
	log.Println("server started on port 8080")
}
