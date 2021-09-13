
// Verifies that all series names are unique
func (c *Catalog) IsSeriesNamesValid() bool {
  valid := true
  
  // extract series names
  names := make([]string, len(c.Series))
  for index, seri := range c.Series {
    names[index] = seri.Name
  }
  sort.Strings(names)
  
  // look for duplicates
  for index := 0; index < len(names)-1; index++ {
    if names[index] == names[index+1] {
      valid = false
      c.printProblem("There are multiple series with the name '%s'", names[index])
    }
  }
  return valid
}

// Verifies that all messages (that are not in a series) have names that are unique
func (c *Catalog) IsMessageNamesValid() bool {
  valid := true
  
  // extract series and message names
  names := []strings{}
  for _, seri := range c.Series {
    names = append(names, seri.Name)
  }
  for _, msg := range c.Messages {
    if len(msg.Series) > 0 {
      continue
    }
    names = append(names, msg.Name)
  }
  sort.Strings(names)
  
  // look for duplicates
  for index := 0; index < len(names)-1; index++ {
    if names[index] == names[index+1 {
      valid = false
      c.printProblem("Message name '%s' conflicts with another message with the same name", names[index])
    }
  }
  
  return valid
}
                             
                             // Creates a new series record from a message. This creates a Series that is a Series of the one message
                             // that was passed in
                             func NewSeriesFromMessage(msg *CatalogMessage) CatalogSeri {
                               seri := CatalogSeri
                               seri.Name = msg.Name
                               seri.Description = msg.Description
                               seri.Resources = msg.Resources
                               seri.Visibility = msg.Visibility
                               seri.StartDate = msg.Date
                               seri.EndDate = msg.Date
                               seri.Messages = []CatalogMessage{*msg}
                               
                               seri.ID = "SAM-" + computeHash(seri.Name)
                             }

                             // Gets the Ministry of a series from the first message in the series
                             func (s *CatalogSeri) GetMinistry() string {
                               if len(s.Messages) > 0 {
                                 return s.Messages[0].Ministry
                               }
                               return UnknownMinistry
                             }
                             
                             // Gets the ID of a series. If the series has an explicit ID (from the spreadsheet) then it will
                             // be returned. If the series doesn't have an ID yet, then one will be created from the name.
                             // Ideally, the ID of a series should be unique and persistent, so this is why we use the ID
                             // from the spreadsheet first (because it should never change). Generating an ID from the name
                             // is second-best because it is only persistent unless somone changes the name
                             func (s *CatalogSeri) GetID() string {
                               if s.ID == "" {
                                 // generate an ID from the name
                                 prefix := "ID-"
                                 switch s.GetMinistry() {
                                   case WordOfLife: prefix = "WOLS-"
                                   case CenterOfRelationshipExperience: prefix = "CORE-"
                                   case AskThePastor: prefix = "ATP-"
                                   case FaithAndFreedom: prefix = "FandF-"
                                 }
                                 s.ID = prefix + computeHash(s.Name)
                               }
                               
                               return s.ID
                             }

                             
                             
