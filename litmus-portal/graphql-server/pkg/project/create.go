package project

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/litmuschaos/litmus/litmus-portal/graphql-server/pkg/myhub"

	"github.com/google/uuid"

	"github.com/litmuschaos/litmus/litmus-portal/graphql-server/graph/model"
	dbOperationsProject "github.com/litmuschaos/litmus/litmus-portal/graphql-server/pkg/database/mongodb/project"
	dbSchemaProject "github.com/litmuschaos/litmus/litmus-portal/graphql-server/pkg/database/mongodb/project"
	dbOperationsUserManagement "github.com/litmuschaos/litmus/litmus-portal/graphql-server/pkg/database/mongodb/usermanagement"
	dbSchemaUserManagement "github.com/litmuschaos/litmus/litmus-portal/graphql-server/pkg/database/mongodb/usermanagement"
)

// CreateProjectWithUser ...
func CreateProjectWithUser(ctx context.Context, projectName string, user *dbSchemaUserManagement.User) (*model.Project, error) {

	uuid := uuid.New()
	newProject := &dbSchemaProject.Project{
		ID:   uuid.String(),
		Name: projectName,
		Members: []*dbSchemaProject.Member{
			{
				UserID:     user.ID,
				UserName:   user.Username,
				Name:       *user.Name,
				Email:      *user.Email,
				Role:       model.MemberRoleOwner,
				Invitation: dbSchemaProject.AcceptedInvitation,
				JoinedAt:   time.Now().Format(time.RFC1123Z),
			},
		},
		CreatedAt: time.Now().String(),
	}

	err := dbOperationsProject.CreateProject(ctx, newProject)
	if err != nil {
		log.Print("ERROR", err)
		return nil, err
	}

	defaultHub := model.CreateMyHub{
		HubName:    "Chaos Hub",
		RepoURL:    "https://github.com/litmuschaos/chaos-charts",
		RepoBranch: "master",
	}

	log.Print("Cloning https://github.com/litmuschaos/chaos-charts")
	go myhub.AddMyHub(context.Background(), defaultHub, newProject.ID)

	return newProject.GetOutputProject(), nil
}

// GetProject ...
func GetProject(ctx context.Context, projectID string) (*model.Project, error) {
	project, err := dbOperationsProject.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return project.GetOutputProject(), nil
}

// GetProjectsByUserID ...
func GetProjectsByUserID(ctx context.Context, userID string) ([]*model.Project, error) {
	projects, err := dbOperationsProject.GetProjectsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	outputProjects := []*model.Project{}
	for _, project := range projects {
		outputProjects = append(outputProjects, project.GetOutputProject())
	}
	return outputProjects, nil
}

// SendInvitation :Send an invitation
func SendInvitation(ctx context.Context, member model.MemberInput) (*model.Member, error) {
	invitation, err := getInvitation(ctx, member)
	if err != nil {
		return nil, err
	}

	if invitation == dbSchemaProject.AcceptedInvitation {
		return nil, errors.New("This user is already a member of this project")
	} else if invitation == dbSchemaProject.PendingInvitation || invitation == dbSchemaProject.DeclinedInvitation || invitation == dbSchemaProject.ExitedProject {
		err = dbOperationsProject.UpdateInvite(ctx, member.ProjectID, member.UserID, dbSchemaProject.PendingInvitation, member.Role)
		if err != nil {
			return nil, errors.New("Unsuccessful")
		}
		return nil, err
	}

	user, err := dbOperationsUserManagement.GetUserByUserID(ctx, member.UserID)
	if err != nil {
		return nil, err
	}
	newMember := &dbSchemaProject.Member{
		UserID:     user.ID,
		UserName:   user.Username,
		Name:       *user.Name,
		Email:      *user.Email,
		Role:       *member.Role,
		Invitation: dbSchemaProject.PendingInvitation,
	}

	err = dbOperationsProject.AddMember(ctx, member.ProjectID, newMember)
	return newMember.GetOutputMember(), err
}

// AcceptInvitation :Accept an invitaion
func AcceptInvitation(ctx context.Context, member model.MemberInput) (string, error) {

	invitation, err := getInvitation(ctx, member)
	if err != nil {
		return "Unsuccessful", err
	}

	if invitation == dbSchemaProject.AcceptedInvitation {
		return "Unsuccessful", errors.New("You are already a member of this project")
	} else if invitation == dbSchemaProject.PendingInvitation {
		err := dbOperationsProject.UpdateInvite(ctx, member.ProjectID, member.UserID, dbSchemaProject.AcceptedInvitation, nil)
		if err != nil {
			return "Unsuccessful", err
		}
		return "Successfull", nil
	} else if invitation == dbSchemaProject.DeclinedInvitation {
		return "Unsuccessful", errors.New("You have already declined the request")
	} else if invitation == dbSchemaProject.ExitedProject {
		return "Unsuccessful", errors.New("You are no longer a member of this project")
	}

	return "Unsuccessful", errors.New("No invitation is present to accept")
}

// DeclineInvitation :Decline an Invitaion
func DeclineInvitation(ctx context.Context, member model.MemberInput) (string, error) {

	invitation, err := getInvitation(ctx, member)
	if err != nil {
		return "Unsuccessful", err
	}

	if invitation == dbSchemaProject.AcceptedInvitation {
		return "Unsuccessful", errors.New("You are already a member of this project")
	} else if invitation == dbSchemaProject.PendingInvitation {
		err := dbOperationsProject.UpdateInvite(ctx, member.ProjectID, member.UserID, dbSchemaProject.DeclinedInvitation, nil)
		if err != nil {
			return "Unsuccessful", err
		}
		return "Successfull", nil
	} else if invitation == dbSchemaProject.DeclinedInvitation {
		return "Unsuccessful", errors.New("You have already declined the request")
	} else if invitation == dbSchemaProject.ExitedProject {
		return "Unsuccessful", errors.New("You are no longer a member of this project")
	}

	return "Unsuccessful", errors.New("No invitation is present to decline")
}

//LeaveProject :Leave a Project
func LeaveProject(ctx context.Context, member model.MemberInput) (string, error) {
	invitation, err := getInvitation(ctx, member)
	if err != nil {
		return "Unsuccessful", err
	}

	if invitation == dbSchemaProject.AcceptedInvitation {
		err := dbOperationsProject.UpdateInvite(ctx, member.ProjectID, member.UserID, dbSchemaProject.ExitedProject, nil)
		if err != nil {
			return "Unsuccessful", err
		}
		return "Successfull", err
	} else if invitation == dbSchemaProject.PendingInvitation || invitation == dbSchemaProject.DeclinedInvitation || invitation == dbSchemaProject.ExitedProject {
		return "Unsuccessful", errors.New("You are not a member of this project")
	}

	return "Unsuccessful", errors.New("No Project to perform this operation on (Leave Project)")
}

// getInvitation :Returns the Invitation Status
func getInvitation(ctx context.Context, member model.MemberInput) (dbSchemaProject.Invitation, error) {

	project, err := dbOperationsProject.GetProject(ctx, member.ProjectID)
	if err != nil {
		return "", err
	}
	for _, projectMember := range project.Members {
		if projectMember.UserID == member.UserID {
			return projectMember.Invitation, nil
		}
	}

	return "", nil
}

// RemoveInvitation :Removes member or cancels invitation
func RemoveInvitation(ctx context.Context, member model.MemberInput) (string, error) {

	invitation, err := getInvitation(ctx, member)
	if err != nil {
		return "Unsuccessful", err
	}
	if invitation == dbSchemaProject.AcceptedInvitation || invitation == dbSchemaProject.PendingInvitation {
		er := dbOperationsProject.RemoveInvitation(ctx, member.ProjectID, member.UserID, invitation)
		if er != nil {
			return "Unsuccessful", er
		}
	} else if invitation == dbSchemaProject.DeclinedInvitation {
		return "Unsuccessful", errors.New("User has already declined the invitation")
	} else if invitation == dbSchemaProject.ExitedProject {
		return "Unsuccessful", errors.New("User is no longer a member of this project")
	}
	return "Successful", nil
}
