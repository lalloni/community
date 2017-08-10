// Copyright 2016 Documize Inc. <legal@documize.com>. All rights reserved.
//
// This software (Documize Community Edition) is licensed under
// GNU AGPL v3 http://www.gnu.org/licenses/agpl-3.0.en.html
//
// You can operate outside the AGPL restrictions by purchasing
// Documize Enterprise Edition and obtaining a commercial license
// by contacting <sales@documize.com>.
//
// https://documize.com

package mysql

import (
	"fmt"
	"time"

	"github.com/documize/community/core/env"
	"github.com/documize/community/core/streamutil"
	"github.com/documize/community/core/uniqueid"
	"github.com/documize/community/domain"
	"github.com/documize/community/domain/store/mysql"
	"github.com/documize/community/model/link"
	"github.com/pkg/errors"
)

// Scope provides data access to MySQL.
type Scope struct {
	Runtime *env.Runtime
}

// Add inserts wiki-link into the store.
// These links exist when content references another document or content.
func (s Scope) Add(ctx domain.RequestContext, l link.Link) (err error) {
	l.Created = time.Now().UTC()
	l.Revised = time.Now().UTC()

	stmt, err := ctx.Transaction.Preparex("INSERT INTO link (refid, orgid, folderid, userid, sourcedocumentid, sourcepageid, targetdocumentid, targetid, linktype, orphan, created, revised) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	defer streamutil.Close(stmt)

	if err != nil {
		err = errors.Wrap(err, "prepare link insert")
		return
	}

	_, err = stmt.Exec(l.RefID, l.OrgID, l.FolderID, l.UserID, l.SourceDocumentID, l.SourcePageID, l.TargetDocumentID, l.TargetID, l.LinkType, l.Orphan, l.Created, l.Revised)
	if err != nil {
		err = errors.Wrap(err, "execute link insert")
		return
	}

	return
}

// GetDocumentOutboundLinks returns outbound links for specified document.
func (s Scope) GetDocumentOutboundLinks(ctx domain.RequestContext, documentID string) (links []link.Link, err error) {
	err = s.Runtime.Db.Select(&links,
		`select l.refid, l.orgid, l.folderid, l.userid, l.sourcedocumentid, l.sourcepageid, l.targetdocumentid, l.targetid, l.linktype, l.orphan, l.created, l.revised
		FROM link l
		WHERE l.orgid=? AND l.sourcedocumentid=?`,
		ctx.OrgID,
		documentID)

	if err != nil {
		return
	}

	if len(links) == 0 {
		links = []link.Link{}
	}

	return
}

// GetPageLinks returns outbound links for specified page in document.
func (s Scope) GetPageLinks(ctx domain.RequestContext, documentID, pageID string) (links []link.Link, err error) {
	err = s.Runtime.Db.Select(&links,
		`select l.refid, l.orgid, l.folderid, l.userid, l.sourcedocumentid, l.sourcepageid, l.targetdocumentid, l.targetid, l.linktype, l.orphan, l.created, l.revised
		FROM link l
		WHERE l.orgid=? AND l.sourcedocumentid=? AND l.sourcepageid=?`,
		ctx.OrgID,
		documentID,
		pageID)

	if err != nil {
		return
	}

	if len(links) == 0 {
		links = []link.Link{}
	}

	return
}

// MarkOrphanDocumentLink marks all link records referencing specified document.
func (s Scope) MarkOrphanDocumentLink(ctx domain.RequestContext, documentID string) (err error) {
	revised := time.Now().UTC()

	stmt, err := ctx.Transaction.Preparex("UPDATE link SET orphan=1, revised=? WHERE linktype='document' AND orgid=? AND targetdocumentid=?")
	defer streamutil.Close(stmt)

	if err != nil {
		return
	}

	_, err = stmt.Exec(revised, ctx.OrgID, documentID)

	return
}

// MarkOrphanPageLink marks all link records referencing specified page.
func (s Scope) MarkOrphanPageLink(ctx domain.RequestContext, pageID string) (err error) {
	revised := time.Now().UTC()

	stmt, err := ctx.Transaction.Preparex("UPDATE link SET orphan=1, revised=? WHERE linktype='section' AND orgid=? AND targetid=?")
	defer streamutil.Close(stmt)

	if err != nil {
		return
	}

	_, err = stmt.Exec(revised, ctx.OrgID, pageID)

	return
}

// MarkOrphanAttachmentLink marks all link records referencing specified attachment.
func (s Scope) MarkOrphanAttachmentLink(ctx domain.RequestContext, attachmentID string) (err error) {
	revised := time.Now().UTC()

	stmt, err := ctx.Transaction.Preparex("UPDATE link SET orphan=1, revised=? WHERE linktype='file' AND orgid=? AND targetid=?")
	defer streamutil.Close(stmt)

	if err != nil {
		return
	}

	_, err = stmt.Exec(revised, ctx.OrgID, attachmentID)

	return
}

// DeleteSourcePageLinks removes saved links for given source.
func (s Scope) DeleteSourcePageLinks(ctx domain.RequestContext, pageID string) (rows int64, err error) {
	b := mysql.BaseQuery{}
	return b.DeleteWhere(ctx.Transaction, fmt.Sprintf("DELETE FROM link WHERE orgid=\"%s\" AND sourcepageid=\"%s\"", ctx.OrgID, pageID))
}

// DeleteSourceDocumentLinks removes saved links for given document.
func (s Scope) DeleteSourceDocumentLinks(ctx domain.RequestContext, documentID string) (rows int64, err error) {
	b := mysql.BaseQuery{}
	return b.DeleteWhere(ctx.Transaction, fmt.Sprintf("DELETE FROM link WHERE orgid=\"%s\" AND sourcedocumentid=\"%s\"", ctx.OrgID, documentID))
}

// DeleteLink removes saved link from the store.
func (s Scope) DeleteLink(ctx domain.RequestContext, id string) (rows int64, err error) {
	b := mysql.BaseQuery{}
	return b.DeleteConstrained(ctx.Transaction, "link", ctx.OrgID, id)
}

// SearchCandidates returns matching documents, sections and attachments using keywords.
func (s Scope) SearchCandidates(ctx domain.RequestContext, keywords string) (docs []link.Candidate,
	pages []link.Candidate, attachments []link.Candidate, err error) {
	// find matching documents
	temp := []link.Candidate{}
	likeQuery := "title LIKE '%" + keywords + "%'"

	err = s.Runtime.Db.Select(&temp,
		`SELECT refid as documentid, labelid as folderid,title from document WHERE orgid=? AND `+likeQuery+` AND labelid IN
		(SELECT refid from label WHERE orgid=? AND type=2 AND userid=?
    	UNION ALL SELECT refid FROM label a where orgid=? AND type=1 AND refid IN (SELECT labelid from labelrole WHERE orgid=? AND userid='' AND (canedit=1 OR canview=1))
		UNION ALL SELECT refid FROM label a where orgid=? AND type=3 AND refid IN (SELECT labelid from labelrole WHERE orgid=? AND userid=? AND (canedit=1 OR canview=1)))
		ORDER BY title`,
		ctx.OrgID,
		ctx.OrgID,
		ctx.UserID,
		ctx.OrgID,
		ctx.OrgID,
		ctx.OrgID,
		ctx.OrgID,
		ctx.UserID)

	if err != nil {
		err = errors.Wrap(err, "execute search links 1")
		return
	}

	for _, r := range temp {
		c := link.Candidate{
			RefID:      uniqueid.Generate(),
			FolderID:   r.FolderID,
			DocumentID: r.DocumentID,
			TargetID:   r.DocumentID,
			LinkType:   "document",
			Title:      r.Title,
			Context:    "",
		}

		docs = append(docs, c)
	}

	// find matching sections
	likeQuery = "p.title LIKE '%" + keywords + "%'"
	temp = []link.Candidate{}

	err = s.Runtime.Db.Select(&temp,
		`SELECT p.refid as targetid, p.documentid as documentid, p.title as title, p.pagetype as linktype, d.title as context, d.labelid as folderid from page p
		LEFT JOIN document d ON d.refid=p.documentid WHERE p.orgid=? AND `+likeQuery+` AND d.labelid IN
		(SELECT refid from label WHERE orgid=? AND type=2 AND userid=?
    	UNION ALL SELECT refid FROM label a where orgid=? AND type=1 AND refid IN (SELECT labelid from labelrole WHERE orgid=? AND userid='' AND (canedit=1 OR canview=1))
		UNION ALL SELECT refid FROM label a where orgid=? AND type=3 AND refid IN (SELECT labelid from labelrole WHERE orgid=? AND userid=? AND (canedit=1 OR canview=1)))
		ORDER BY p.title`,
		ctx.OrgID,
		ctx.OrgID,
		ctx.UserID,
		ctx.OrgID,
		ctx.OrgID,
		ctx.OrgID,
		ctx.OrgID,
		ctx.UserID)

	if err != nil {
		err = errors.Wrap(err, "execute search links 2")
		return
	}

	for _, r := range temp {
		c := link.Candidate{
			RefID:      uniqueid.Generate(),
			FolderID:   r.FolderID,
			DocumentID: r.DocumentID,
			TargetID:   r.TargetID,
			LinkType:   r.LinkType,
			Title:      r.Title,
			Context:    r.Context,
		}

		pages = append(pages, c)
	}

	// find matching attachments
	likeQuery = "a.filename LIKE '%" + keywords + "%'"
	temp = []link.Candidate{}

	err = s.Runtime.Db.Select(&temp,
		`SELECT a.refid as targetid, a.documentid as documentid, a.filename as title, a.extension as context, d.labelid as folderid from attachment a
		LEFT JOIN document d ON d.refid=a.documentid WHERE a.orgid=? AND `+likeQuery+` AND d.labelid IN
		(SELECT refid from label WHERE orgid=? AND type=2 AND userid=?
    	UNION ALL SELECT refid FROM label a where orgid=? AND type=1 AND refid IN (SELECT labelid from labelrole WHERE orgid=? AND userid='' AND (canedit=1 OR canview=1))
		UNION ALL SELECT refid FROM label a where orgid=? AND type=3 AND refid IN (SELECT labelid from labelrole WHERE orgid=? AND userid=? AND (canedit=1 OR canview=1)))
		ORDER BY a.filename`,
		ctx.OrgID,
		ctx.OrgID,
		ctx.UserID,
		ctx.OrgID,
		ctx.OrgID,
		ctx.OrgID,
		ctx.OrgID,
		ctx.UserID)

	if err != nil {
		err = errors.Wrap(err, "execute search links 3")
		return
	}

	for _, r := range temp {
		c := link.Candidate{
			RefID:      uniqueid.Generate(),
			FolderID:   r.FolderID,
			DocumentID: r.DocumentID,
			TargetID:   r.TargetID,
			LinkType:   "file",
			Title:      r.Title,
			Context:    r.Context,
		}

		attachments = append(attachments, c)
	}

	if len(docs) == 0 {
		docs = []link.Candidate{}
	}
	if len(pages) == 0 {
		pages = []link.Candidate{}
	}
	if len(attachments) == 0 {
		attachments = []link.Candidate{}
	}

	return
}